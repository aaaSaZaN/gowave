package stubs

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"remnawave-go/config"
	"remnawave-go/internal/database"
	"remnawave-go/internal/httpx"
	"remnawave-go/internal/subscription"

	"github.com/go-chi/chi/v5"
)

// SetConfig supplies the environment-derived settings exposed by
// /api/system/configuration.
func (s *StubHandler) SetConfig(cfg *config.Config) { s.cfg = cfg }

func nullableList(values []int) interface{} {
	if values == nil {
		return nil
	}
	return values
}

func (s *StubHandler) GetSystemConfiguration(w http.ResponseWriter, r *http.Request) {
	cfg := s.cfg
	if cfg == nil {
		cfg = &config.Config{ShortUUIDLength: 16, SubPublicDomain: s.subPublicDomain}
	}
	n := cfg.Notifications
	httpx.Response(w, map[string]interface{}{
		"notifications": map[string]interface{}{
			"webhook":                 n.WebhookEnabled,
			"bandwidthUsage":          nullableList(n.BandwidthUsageThresholds),
			"notConnectedAfter":       nullableList(n.NotConnectedAfterHours),
			"expirationNotifications": nullableList(n.ExpirationNotifications),
		},
		"service": map[string]bool{
			"cleanUsageHistory":       cfg.Service.CleanUsageHistory,
			"disableUserUsageRecords": cfg.Service.DisableUserUsageRecords,
			"disableSrhRecords":       cfg.Service.DisableSrhRecords,
			"exportToRedisStream":     cfg.Service.ExportToStream,
		},
		"misc": map[string]interface{}{
			"shortUuidLength":           cfg.ShortUUIDLength,
			"subPublicDomain":           cfg.SubPublicDomain,
			"userUsageIgnoreBelowBytes": cfg.UserUsageIgnoreBelowBytes,
		},
	})
}

func (s *StubHandler) GetStatsDigest(w http.ResponseWriter, r *http.Request) {
	start, startErr := time.Parse(time.RFC3339Nano, r.URL.Query().Get("start"))
	end, endErr := time.Parse(time.RFC3339Nano, r.URL.Query().Get("end"))
	if startErr != nil || endErr != nil {
		httpx.Error(w, r, http.StatusBadRequest, "E000", "start and end must be ISO 8601 datetimes with a timezone")
		return
	}
	start, end = start.UTC(), end.UTC()
	if !start.Before(end) {
		httpx.Error(w, r, http.StatusBadRequest, httpx.CodeInvalidRange, "Start date must be before or equal to end date")
		return
	}
	var users struct {
		CreatedCount int64
		ExpiredCount int64
	}
	var totalBytes, newUsersBytes, devices int64
	db := s.db
	err := db.Model(&database.User{}).Select(
		"COALESCE(SUM(CASE WHEN created_at >= ? AND created_at < ? THEN 1 ELSE 0 END), 0) AS created_count, "+
			"COALESCE(SUM(CASE WHEN expire_at >= ? AND expire_at < ? THEN 1 ELSE 0 END), 0) AS expired_count",
		start, end, start, end).Scan(&users).Error
	if err == nil {
		// Upstream sums createdAt in [start, end - 1ms].
		err = db.Model(&database.NodesUsageHistory{}).Where("created_at >= ? AND created_at <= ?", start, end.Add(-time.Millisecond)).
			Select("COALESCE(SUM(total_bytes), 0)").Scan(&totalBytes).Error
	}
	if err == nil {
		err = db.Model(&database.NodesUserUsageHistory{}).
			Where("created_at >= ? AND created_at < ?", start, end).
			Where("user_id IN (?)", db.Model(&database.User{}).Select("id").Where("created_at >= ? AND created_at < ?", start, end)).
			Select("COALESCE(SUM(total_bytes), 0)").Scan(&newUsersBytes).Error
	}
	if err == nil {
		err = db.Model(&database.HwidDevice{}).Where("created_at >= ? AND created_at < ?", start, end).Count(&devices).Error
	}
	if err != nil {
		httpx.Error(w, r, http.StatusInternalServerError, "A236", "Get stats digest error")
		return
	}
	httpx.Response(w, map[string]interface{}{
		"users":       map[string]int64{"createdCount": users.CreatedCount, "expiredCount": users.ExpiredCount},
		"traffic":     map[string]string{"totalBytes": strconv.FormatInt(totalBytes, 10), "byUsersCreatedInRangeBytes": strconv.FormatInt(newUsersBytes, 10)},
		"hwidDevices": map[string]int64{"createdCount": devices},
	})
}

// GenerateX25519 returns 30 Reality key pairs as unpadded URL-safe base64,
// matching upstream's @stablelib/x25519 generator.
func (s *StubHandler) GenerateX25519(w http.ResponseWriter, r *http.Request) {
	keypairs := make([]map[string]string, 0, 30)
	for i := 0; i < 30; i++ {
		key, err := ecdh.X25519().GenerateKey(rand.Reader)
		if err != nil {
			httpx.Error(w, r, http.StatusInternalServerError, httpx.CodeInternal, "Server error")
			return
		}
		keypairs = append(keypairs, map[string]string{
			"publicKey":  base64.RawURLEncoding.EncodeToString(key.PublicKey().Bytes()),
			"privateKey": base64.RawURLEncoding.EncodeToString(key.Bytes()),
		})
	}
	httpx.Response(w, map[string]interface{}{"keypairs": keypairs})
}

// DebugSrrMatcher evaluates the submitted response rules against this
// request's own headers, as upstream's tester does.
func (s *StubHandler) DebugSrrMatcher(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ResponseRules json.RawMessage `json:"responseRules"`
	}
	if err := httpx.Decode(r, 1<<20, &body); err != nil || len(body.ResponseRules) == 0 {
		httpx.Error(w, r, http.StatusBadRequest, "E000", "responseRules is required")
		return
	}
	headers := map[string]string{}
	inputHeaders := map[string]string{}
	for name, values := range r.Header {
		key := strings.ToLower(name)
		headers[key] = strings.Join(values, ",")
		inputHeaders[key] = headers[key]
	}
	if r.Host != "" {
		headers["host"], inputHeaders["host"] = r.Host, r.Host
	}
	result := subscription.MatchSRRRules(string(body.ResponseRules), headers, "")
	outputHeaders := []subscription.SRRHeaderMod{}
	var matchedRule interface{}
	if result.MatchedRule != nil {
		matchedRule = result.MatchedRule
		if mods := result.MatchedRule.ResponseModifications; mods != nil && mods.Headers != nil {
			outputHeaders = mods.Headers
		}
	}
	response := map[string]interface{}{
		"matched":       result.Matched,
		"matchedRule":   matchedRule,
		"inputHeaders":  inputHeaders,
		"outputHeaders": outputHeaders,
	}
	if result.ResponseType != "" {
		response["responseType"] = result.ResponseType
	}
	httpx.Response(w, response)
}

// squadNodes lists nodes that serve at least one of the squad's inbounds.
func (s *StubHandler) squadNodes(squad string) (map[uint64]string, error) {
	var rows []struct {
		ID   uint64
		UUID string
	}
	err := s.db.Table("nodes AS n").
		Joins("JOIN config_profile_inbounds AS cpi ON cpi.profile_uuid = n.active_config_profile_uuid").
		Joins("JOIN config_profile_inbounds_to_nodes AS cpin ON cpin.config_profile_inbound_uuid = cpi.uuid AND cpin.node_uuid = n.uuid").
		Joins("JOIN internal_squad_inbounds AS isi ON isi.inbound_uuid = cpi.uuid").
		Where("isi.internal_squad_uuid = ?", squad).
		Distinct("n.id", "n.uuid").Scan(&rows).Error
	result := make(map[uint64]string, len(rows))
	for _, row := range rows {
		result[row.ID] = row.UUID
	}
	return result, err
}

// dayRange parses YYYY-MM-DD bounds into [start of start day, end of end day].
func dayRange(r *http.Request) (time.Time, time.Time, bool) {
	start, err1 := time.Parse("2006-01-02", r.URL.Query().Get("start"))
	end, err2 := time.Parse("2006-01-02", r.URL.Query().Get("end"))
	if err1 != nil || err2 != nil || end.Before(start) {
		return time.Time{}, time.Time{}, false
	}
	return start, end.Add(24*time.Hour - time.Millisecond), true
}

func (s *StubHandler) squadExists(w http.ResponseWriter, r *http.Request, squad string) bool {
	var count int64
	if err := s.db.Model(&database.InternalSquad{}).Where("uuid = ?", squad).Count(&count).Error; err != nil || count == 0 {
		httpx.Error(w, r, http.StatusNotFound, httpx.CodeSquadNotFound, "Internal squad not found")
		return false
	}
	return true
}

func (s *StubHandler) GetBandwidthSquadUsage(w http.ResponseWriter, r *http.Request) {
	squad := chi.URLParam(r, "uuid")
	if !s.squadExists(w, r, squad) {
		return
	}
	start, end, ok := dayRange(r)
	if !ok {
		httpx.Error(w, r, http.StatusBadRequest, "E000", "start and end must be YYYY-MM-DD dates")
		return
	}
	query := r.URL.Query()
	minTotal, _ := strconv.ParseInt(query.Get("minTotalBytes"), 10, 64)
	limit, err := strconv.Atoi(query.Get("limit"))
	if err != nil || limit < 1 {
		limit = 250
	}
	if limit > 1000 {
		limit = 1000
	}
	empty := map[string]interface{}{"squadUuid": squad, "users": []interface{}{}, "nextCursor": nil, "hasMore": false}
	nodes, err := s.squadNodes(squad)
	if err != nil {
		httpx.Error(w, r, http.StatusInternalServerError, "A232", "Get internal squad usage error")
		return
	}
	if len(nodes) == 0 {
		httpx.Response(w, empty)
		return
	}
	nodeIDs := make([]uint64, 0, len(nodes))
	for id := range nodes {
		nodeIDs = append(nodeIDs, id)
	}
	builder := s.db.Table("internal_squad_members AS m").
		Joins("JOIN nodes_user_usage_history AS h ON h.user_id = m.user_id").
		Where("m.internal_squad_uuid = ? AND h.created_at >= ? AND h.created_at <= ? AND h.node_id IN ?", squad, start, end, nodeIDs)
	if cursor, err := strconv.ParseUint(query.Get("cursor"), 10, 64); err == nil && cursor > 0 {
		builder = builder.Where("m.user_id > ?", cursor)
	}
	var rows []struct {
		ID         uint64
		TotalBytes int64
	}
	if err := builder.Select("m.user_id AS id, SUM(h.total_bytes) AS total_bytes").Group("m.user_id").
		Having("SUM(h.total_bytes) >= ?", minTotal).Order("m.user_id ASC").Limit(limit + 1).Scan(&rows).Error; err != nil {
		httpx.Error(w, r, http.StatusInternalServerError, "A232", "Get internal squad usage error")
		return
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	users := make([]map[string]interface{}, 0, len(rows))
	for _, row := range rows {
		users = append(users, map[string]interface{}{"id": row.ID, "totalBytes": row.TotalBytes})
	}
	var nextCursor interface{}
	if hasMore {
		nextCursor = strconv.FormatUint(rows[len(rows)-1].ID, 10)
	}
	httpx.Response(w, map[string]interface{}{"squadUuid": squad, "users": users, "nextCursor": nextCursor, "hasMore": hasMore})
}

func (s *StubHandler) GetBandwidthSquadUserUsage(w http.ResponseWriter, r *http.Request) {
	squad := chi.URLParam(r, "squadUuid")
	if !s.squadExists(w, r, squad) {
		return
	}
	userID, err := strconv.ParseUint(chi.URLParam(r, "userId"), 10, 64)
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, "E000", "userId must be a number")
		return
	}
	start, end, ok := dayRange(r)
	if !ok {
		httpx.Error(w, r, http.StatusBadRequest, "E000", "start and end must be YYYY-MM-DD dates")
		return
	}
	var dates []string
	for day := start; !day.After(end); day = day.Add(24 * time.Hour) {
		dates = append(dates, day.Format("2006-01-02"))
	}
	nodes, err := s.squadNodes(squad)
	if err != nil {
		httpx.Error(w, r, http.StatusInternalServerError, "A232", "Get internal squad usage error")
		return
	}
	byDate := map[string][]map[string]interface{}{}
	if len(nodes) > 0 {
		nodeIDs := make([]uint64, 0, len(nodes))
		for id := range nodes {
			nodeIDs = append(nodeIDs, id)
		}
		var rows []database.NodesUserUsageHistory
		if err := s.db.Where("user_id = ? AND created_at >= ? AND created_at <= ? AND node_id IN ?", userID, start, end, nodeIDs).
			Find(&rows).Error; err != nil {
			httpx.Error(w, r, http.StatusInternalServerError, "A232", "Get internal squad usage error")
			return
		}
		for _, row := range rows {
			date := row.CreatedAt.UTC().Format("2006-01-02")
			byDate[date] = append(byDate[date], map[string]interface{}{"uuid": nodes[row.NodeID], "totalBytes": row.TotalBytes})
		}
	}
	days := make([]map[string]interface{}, 0, len(dates))
	for _, date := range dates {
		entries := byDate[date]
		if entries == nil {
			entries = []map[string]interface{}{}
		}
		days = append(days, map[string]interface{}{"date": date, "nodes": entries})
	}
	httpx.Response(w, map[string]interface{}{"days": days})
}
