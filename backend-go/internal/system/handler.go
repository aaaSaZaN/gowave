package system

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"sort"
	"strings"
	"time"

	"gowave/internal/database"
	"gowave/internal/httpx"
	"gowave/internal/prometheus"

	"gorm.io/gorm"
)

var (
	AppVersion = "v1.0.5"
	BuildTime  = "unknown"
	BuildNum   = "1"
)
var startTime = time.Now()

type Handler struct {
	db      *gorm.DB
	restore *BackupRestoreService
	// OnRestored runs after a backup is published, to rebuild caches that
	// depend on restored rows (node mTLS client, synced users).
	OnRestored func()
}

func NewHandler(db *gorm.DB) *Handler {
	return &Handler{
		db:      db,
		restore: NewBackupRestoreService(db),
	}
}

type BaseStat struct {
	Current    string `json:"current"`
	Previous   string `json:"previous"`
	Difference string `json:"difference"`
}

type BandwidthStatsResponse struct {
	Response struct {
		BandwidthLastTwoDays   BaseStat `json:"bandwidthLastTwoDays"`
		BandwidthLastSevenDays BaseStat `json:"bandwidthLastSevenDays"`
		BandwidthLast30Days    BaseStat `json:"bandwidthLast30Days"`
		BandwidthCalendarMonth BaseStat `json:"bandwidthCalendarMonth"`
		BandwidthCurrentYear   BaseStat `json:"bandwidthCurrentYear"`
	} `json:"response"`
}

type SystemStatsResponse struct {
	Response struct {
		Cpu struct {
			Cores int `json:"cores"`
		} `json:"cpu"`
		Memory struct {
			Total uint64 `json:"total"`
			Free  uint64 `json:"free"`
			Used  uint64 `json:"used"`
		} `json:"memory"`
		Uptime    int64 `json:"uptime"`
		Timestamp int64 `json:"timestamp"`
		Users     struct {
			StatusCounts map[string]int `json:"statusCounts"`
			TotalUsers   int            `json:"totalUsers"`
		} `json:"users"`
		OnlineStats struct {
			LastDay     int `json:"lastDay"`
			LastWeek    int `json:"lastWeek"`
			NeverOnline int `json:"neverOnline"`
			OnlineNow   int `json:"onlineNow"`
		} `json:"onlineStats"`
		Nodes struct {
			TotalOnline        int    `json:"totalOnline"`
			TotalBytesLifetime string `json:"totalBytesLifetime"`
		} `json:"nodes"`
	} `json:"response"`
}

type NodesStatisticsResponse struct {
	Response struct {
		LastSevenDays []NodeDayStat `json:"lastSevenDays"`
	} `json:"response"`
}

type NodeDayStat struct {
	NodeName   string `json:"nodeName" gorm:"column:node_name"`
	Date       string `json:"date" gorm:"column:date"`
	TotalBytes string `json:"totalBytes" gorm:"column:total_bytes"`
}

type RecapResponse struct {
	Response struct {
		ThisMonth struct {
			Users   int    `json:"users"`
			Traffic string `json:"traffic"`
		} `json:"thisMonth"`
		Total struct {
			Users             int    `json:"users"`
			Nodes             int    `json:"nodes"`
			Traffic           string `json:"traffic"`
			NodesRam          string `json:"nodesRam"`
			NodesCpuCores     int    `json:"nodesCpuCores"`
			DistinctCountries int    `json:"distinctCountries"`
		} `json:"total"`
		Version  string `json:"version"`
		InitDate string `json:"initDate"`
	} `json:"response"`
}

type SettingsResponse struct {
	Response interface{} `json:"response"`
}

type MetadataResponse struct {
	Response struct {
		Version string `json:"version"`
		Build   struct {
			Time   string `json:"time"`
			Number string `json:"number"`
		} `json:"build"`
		Git struct {
			Backend struct {
				CommitSha string `json:"commitSha"`
				Branch    string `json:"branch"`
				CommitURL string `json:"commitUrl"`
			} `json:"backend"`
			Frontend struct {
				CommitSha string `json:"commitSha"`
				CommitURL string `json:"commitUrl"`
			} `json:"frontend"`
		} `json:"git"`
	} `json:"response"`
}

func formatPrettyBytes(b int64) string {
	if b == 0 {
		return "0"
	}
	isNeg := b < 0
	if isNeg {
		b = -b
	}
	var res string
	const unit = 1024
	if b < unit {
		res = fmt.Sprintf("%.2f B", float64(b))
	} else {
		div, exp := int64(unit), 0
		units := []string{"KiB", "MiB", "GiB", "TiB", "PiB"}
		for n := b / unit; n >= unit && exp < len(units)-1; n /= unit {
			div *= unit
			exp++
		}
		val := float64(b) / float64(div)
		res = fmt.Sprintf("%.2f %s", val, units[exp])
	}
	if isNeg {
		return "-" + res
	}
	return res
}

func calcBaseStat(cur, prev int64) BaseStat {
	diff := cur - prev
	diffStr := "0"
	if diff != 0 {
		diffStr = formatPrettyBytes(diff)
	}
	return BaseStat{
		Current:    formatPrettyBytes(cur),
		Previous:   formatPrettyBytes(prev),
		Difference: diffStr,
	}
}

func (h *Handler) GetMetadata(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var resp MetadataResponse
	resp.Response.Version = AppVersion
	resp.Response.Build.Time = BuildTime
	resp.Response.Build.Number = BuildNum
	resp.Response.Git.Backend.Branch = "main"

	json.NewEncoder(w).Encode(resp)
}

func formatSubscriptionSettings(s *database.SubscriptionSetting) map[string]interface{} {
	customRemarks := json.RawMessage("{}")
	if s.CustomRemarks != "" && s.CustomRemarks != "null" {
		customRemarks = json.RawMessage(s.CustomRemarks)
	}

	customHeaders := json.RawMessage("{}")
	if s.CustomResponseHeaders != "" && s.CustomResponseHeaders != "null" {
		customHeaders = json.RawMessage(s.CustomResponseHeaders)
	}

	responseRules := json.RawMessage(`{"version":"1","rules":[]}`)
	if s.ResponseRules != "" && s.ResponseRules != "null" {
		responseRules = json.RawMessage(s.ResponseRules)
	}

	hwidSettings := json.RawMessage(`{"enabled":false,"fallbackDeviceLimit":0,"maxDevicesAnnounce":null}`)
	if s.HwidSettings != "" && s.HwidSettings != "null" {
		hwidSettings = json.RawMessage(s.HwidSettings)
	}

	return map[string]interface{}{
		"uuid":                        s.UUID,
		"serveJsonAtBaseSubscription": s.ServeJsonAtBaseSubscription,
		"isShowCustomRemarks":         s.IsShowCustomRemarks,
		"customRemarks":               customRemarks,
		"customResponseHeaders":       customHeaders,
		"randomizeHosts":              s.RandomizeHosts,
		"responseRules":               responseRules,
		"hwidSettings":                hwidSettings,
		"createdAt":                   s.CreatedAt.UTC().Format("2006-01-02T15:04:05.000Z"),
		"updatedAt":                   s.UpdatedAt.UTC().Format("2006-01-02T15:04:05.000Z"),
	}
}

func (h *Handler) GetSubscriptionSettings(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	s := database.EnsureSubscriptionSetting(h.db)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": formatSubscriptionSettings(s),
	})
}

func (h *Handler) UpdateSubscriptionSettings(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	s := database.EnsureSubscriptionSetting(h.db)

	var body struct {
		UUID                        *string         `json:"uuid"`
		ServeJsonAtBaseSubscription *bool           `json:"serveJsonAtBaseSubscription"`
		IsShowCustomRemarks         *bool           `json:"isShowCustomRemarks"`
		CustomRemarks               json.RawMessage `json:"customRemarks"`
		CustomResponseHeaders       json.RawMessage `json:"customResponseHeaders"`
		RandomizeHosts              *bool           `json:"randomizeHosts"`
		ResponseRules               json.RawMessage `json:"responseRules"`
		HwidSettings                json.RawMessage `json:"hwidSettings"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Invalid body"})
		return
	}

	if body.ServeJsonAtBaseSubscription != nil {
		s.ServeJsonAtBaseSubscription = *body.ServeJsonAtBaseSubscription
	}
	if body.IsShowCustomRemarks != nil {
		s.IsShowCustomRemarks = *body.IsShowCustomRemarks
	}
	if len(body.CustomRemarks) > 0 && string(body.CustomRemarks) != "null" {
		s.CustomRemarks = string(body.CustomRemarks)
	}
	if len(body.CustomResponseHeaders) > 0 && string(body.CustomResponseHeaders) != "null" {
		s.CustomResponseHeaders = string(body.CustomResponseHeaders)
	}
	if body.RandomizeHosts != nil {
		s.RandomizeHosts = *body.RandomizeHosts
	}
	if len(body.ResponseRules) > 0 && string(body.ResponseRules) != "null" {
		s.ResponseRules = string(body.ResponseRules)
	}
	if len(body.HwidSettings) > 0 && string(body.HwidSettings) != "null" {
		s.HwidSettings = string(body.HwidSettings)
	}
	s.UpdatedAt = time.Now().UTC()
	h.db.Save(s)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": formatSubscriptionSettings(s),
	})
}

func (h *Handler) GetHttpStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	total, routes := GlobalRouteCounter.GetStats()
	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": map[string]interface{}{
			"routes": routes,
			"total":  total,
		},
	})
}

func (h *Handler) GetHealth(w http.ResponseWriter, r *http.Request) {
	httpx.Response(w, map[string]interface{}{"runtimeMetrics": []prometheus.RuntimeMetric{prometheus.CurrentRuntimeMetric()}})
}

func (h *Handler) GetStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var statusRows []struct {
		Status string
		Count  int
	}
	if err := h.db.Model(&database.User{}).Select("status, COUNT(*) AS count").Group("status").Scan(&statusRows).Error; err != nil {
		http.Error(w, "failed to load user statistics", http.StatusInternalServerError)
		return
	}

	statusCounts := map[string]int{
		"ACTIVE":   0,
		"DISABLED": 0,
		"LIMITED":  0,
		"EXPIRED":  0,
	}
	totalUsers := 0
	for _, row := range statusRows {
		statusCounts[row.Status] += row.Count
		totalUsers += row.Count
	}

	type OnlineStatsResult struct {
		OnlineNow   int
		LastDay     int
		LastWeek    int
		NeverOnline int
	}
	var osr OnlineStatsResult
	now := time.Now().UTC()
	if err := h.db.Raw(`
SELECT
    COUNT(CASE WHEN ut.online_at >= ? THEN 1 END) AS online_now,
    COUNT(CASE WHEN ut.online_at >= ? THEN 1 END) AS last_day,
    COUNT(CASE WHEN ut.online_at >= ? THEN 1 END) AS last_week,
    COUNT(CASE WHEN ut.online_at IS NULL THEN 1 END) AS never_online
FROM users u
LEFT JOIN user_traffic ut ON ut.id = u.id`,
		now.Add(-30*time.Second), now.Add(-24*time.Hour), now.Add(-7*24*time.Hour)).Scan(&osr).Error; err != nil {
		http.Error(w, "failed to load online statistics", http.StatusInternalServerError)
		return
	}

	var totalBytesLifetime int64
	if err := h.db.Model(&database.NodesUsageHistory{}).Select("COALESCE(SUM(total_bytes), 0)").Scan(&totalBytesLifetime).Error; err != nil {
		http.Error(w, "failed to load node traffic statistics", http.StatusInternalServerError)
		return
	}

	var resp SystemStatsResponse
	resp.Response.Cpu.Cores = runtime.NumCPU()
	resp.Response.Memory.Total, resp.Response.Memory.Free, resp.Response.Memory.Used = hostMemory()
	resp.Response.Uptime = int64(time.Since(startTime).Seconds())
	resp.Response.Timestamp = time.Now().UnixMilli()

	resp.Response.Users.StatusCounts = statusCounts
	resp.Response.Users.TotalUsers = totalUsers

	resp.Response.OnlineStats.LastDay = osr.LastDay
	resp.Response.OnlineStats.LastWeek = osr.LastWeek
	resp.Response.OnlineStats.NeverOnline = osr.NeverOnline
	resp.Response.OnlineStats.OnlineNow = osr.OnlineNow

	var onlineNodes int64
	if err := h.db.Model(&database.Node{}).Where("is_connected = ? AND is_disabled = ?", true, false).Count(&onlineNodes).Error; err != nil {
		http.Error(w, "failed to load node status", http.StatusInternalServerError)
		return
	}
	resp.Response.Nodes.TotalOnline = int(onlineNodes)
	resp.Response.Nodes.TotalBytesLifetime = fmt.Sprintf("%d", totalBytesLifetime)

	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) GetBandwidthStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	tzStr := r.URL.Query().Get("tz")
	loc, err := time.LoadLocation(tzStr)
	if err != nil {
		loc = time.UTC
	}

	now := time.Now().In(loc)

	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc).UTC()
	todayEnd := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 999999999, loc).UTC()
	yesterdayStart := todayStart.AddDate(0, 0, -1)
	yesterdayEnd := todayEnd.AddDate(0, 0, -1)

	last7Start := todayStart.AddDate(0, 0, -6)
	last7End := todayEnd
	prev7Start := last7Start.AddDate(0, 0, -7)
	prev7End := todayEnd.AddDate(0, 0, -7)

	last30Start := todayStart.AddDate(0, 0, -29)
	last30End := todayEnd
	prev30Start := last30Start.AddDate(0, 0, -30)
	prev30End := todayEnd.AddDate(0, 0, -30)

	curMonthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, loc).UTC()
	curMonthEnd := todayEnd
	prevMonthStart := curMonthStart.AddDate(0, -1, 0)
	prevMonthEnd := curMonthStart.Add(-time.Nanosecond)

	curYearStart := time.Date(now.Year(), 1, 1, 0, 0, 0, 0, loc).UTC()
	curYearEnd := todayEnd
	prevYearStart := curYearStart.AddDate(-1, 0, 0)
	prevYearEnd := curYearStart.Add(-time.Nanosecond)

	getUsage := func(start, end time.Time) int64 {
		var total int64
		h.db.Raw("SELECT COALESCE(SUM(total_bytes), 0) FROM nodes_usage_history WHERE created_at >= ? AND created_at <= ?", start, end).Scan(&total)
		return total
	}

	todayUsage := getUsage(todayStart, todayEnd)
	yesterdayUsage := getUsage(yesterdayStart, yesterdayEnd)

	last7Usage := getUsage(last7Start, last7End)
	prev7Usage := getUsage(prev7Start, prev7End)

	last30Usage := getUsage(last30Start, last30End)
	prev30Usage := getUsage(prev30Start, prev30End)

	curMonthUsage := getUsage(curMonthStart, curMonthEnd)
	prevMonthUsage := getUsage(prevMonthStart, prevMonthEnd)

	curYearUsage := getUsage(curYearStart, curYearEnd)
	prevYearUsage := getUsage(prevYearStart, prevYearEnd)

	var resp BandwidthStatsResponse
	resp.Response.BandwidthLastTwoDays = calcBaseStat(todayUsage, yesterdayUsage)
	resp.Response.BandwidthLastSevenDays = calcBaseStat(last7Usage, prev7Usage)
	resp.Response.BandwidthLast30Days = calcBaseStat(last30Usage, prev30Usage)
	resp.Response.BandwidthCalendarMonth = calcBaseStat(curMonthUsage, prevMonthUsage)
	resp.Response.BandwidthCurrentYear = calcBaseStat(curYearUsage, prevYearUsage)

	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) GetNodesStatistics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	type historyRow struct {
		NodeName   string    `gorm:"column:node_name"`
		TotalBytes int64     `gorm:"column:total_bytes"`
		CreatedAt  time.Time `gorm:"column:created_at"`
	}
	since := time.Now().UTC().Add(-7 * 24 * time.Hour)
	var history []historyRow
	if err := h.db.Table("nodes_usage_history AS nu").
		Select("n.name AS node_name, nu.total_bytes, nu.created_at").
		Joins("JOIN nodes AS n ON nu.node_uuid = n.uuid").
		Where("nu.created_at >= ?", since).
		Order("nu.created_at ASC, n.name ASC").Scan(&history).Error; err != nil {
		http.Error(w, "failed to load node history", http.StatusInternalServerError)
		return
	}
	grouped := make(map[string]int64)
	for _, row := range history {
		date := row.CreatedAt.UTC().Format("2006-01-02")
		key := date + "\x00" + row.NodeName
		grouped[key] += row.TotalBytes
	}
	keys := make([]string, 0, len(grouped))
	for key := range grouped {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	rows := make([]NodeDayStat, 0, len(keys))
	for _, key := range keys {
		parts := strings.SplitN(key, "\x00", 2)
		rows = append(rows, NodeDayStat{Date: parts[0], NodeName: parts[1], TotalBytes: fmt.Sprintf("%d", grouped[key])})
	}

	var resp NodesStatisticsResponse
	resp.Response.LastSevenDays = rows
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) GetRecap(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	now := time.Now().UTC()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	var totalUsers int64
	var newUsersThisMonth int64
	if err := h.db.Model(&database.User{}).Count(&totalUsers).Error; err != nil {
		http.Error(w, "failed to load user recap", http.StatusInternalServerError)
		return
	}
	if err := h.db.Model(&database.User{}).Where("created_at >= ?", monthStart).Count(&newUsersThisMonth).Error; err != nil {
		http.Error(w, "failed to load monthly user recap", http.StatusInternalServerError)
		return
	}

	var thisMonthTrafficBytes, totalTrafficBytes int64
	if err := h.db.Model(&database.NodesUsageHistory{}).Select("COALESCE(SUM(total_bytes), 0)").Where("created_at >= ?", monthStart).Scan(&thisMonthTrafficBytes).Error; err != nil {
		http.Error(w, "failed to load monthly traffic recap", http.StatusInternalServerError)
		return
	}
	if err := h.db.Model(&database.NodesUsageHistory{}).Select("COALESCE(SUM(total_bytes), 0)").Scan(&totalTrafficBytes).Error; err != nil {
		http.Error(w, "failed to load total traffic recap", http.StatusInternalServerError)
		return
	}

	var totalNodes int64
	var distinctCountries int64
	if err := h.db.Model(&database.Node{}).Count(&totalNodes).Error; err != nil {
		http.Error(w, "failed to load node recap", http.StatusInternalServerError)
		return
	}
	if err := h.db.Raw(`SELECT COUNT(DISTINCT CASE WHEN country_code IS NOT NULL AND country_code != '' AND country_code != 'XX' THEN country_code END) FROM nodes`).Scan(&distinctCountries).Error; err != nil {
		http.Error(w, "failed to load country recap", http.StatusInternalServerError)
		return
	}

	var initDate time.Time
	if h.db.Migrator().HasTable("_prisma_migrations") {
		_ = h.db.Raw(`SELECT started_at FROM _prisma_migrations ORDER BY started_at ASC LIMIT 1`).Scan(&initDate).Error
	}
	if initDate.IsZero() {
		initDate = startTime
	}

	var resp RecapResponse
	resp.Response.ThisMonth.Users = int(newUsersThisMonth)
	resp.Response.ThisMonth.Traffic = fmt.Sprintf("%d", thisMonthTrafficBytes)
	resp.Response.Total.Users = int(totalUsers)
	resp.Response.Total.Nodes = int(totalNodes)
	resp.Response.Total.Traffic = fmt.Sprintf("%d", totalTrafficBytes)
	resp.Response.Total.NodesRam = "0 B"
	resp.Response.Total.NodesCpuCores = 0
	resp.Response.Total.DistinctCountries = int(distinctCountries)
	resp.Response.Version = AppVersion
	resp.Response.InitDate = initDate.UTC().Format(time.RFC3339)

	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) GetSettings(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var setting database.RemnawaveSetting
	h.db.First(&setting)
	title := "Remnawave"
	if setting.Title != "" {
		title = setting.Title
	}

	passkeySettings := map[string]interface{}{
		"enabled": false,
		"rpId":    nil,
		"origin":  nil,
	}
	if setting.PasskeySettings != "" && setting.PasskeySettings != "{}" {
		_ = json.Unmarshal([]byte(setting.PasskeySettings), &passkeySettings)
	}

	oauth2Settings := map[string]interface{}{
		"github": map[string]interface{}{
			"enabled":       false,
			"clientId":      nil,
			"clientSecret":  nil,
			"allowedEmails": []string{},
		},
		"pocketid": map[string]interface{}{
			"enabled":        false,
			"clientId":       nil,
			"clientSecret":   nil,
			"frontendDomain": nil,
			"plainDomain":    nil,
			"allowedEmails":  []string{},
		},
		"yandex": map[string]interface{}{
			"enabled":       false,
			"clientId":      nil,
			"clientSecret":  nil,
			"allowedEmails": []string{},
		},
		"keycloak": map[string]interface{}{
			"enabled":        false,
			"realm":          nil,
			"clientId":       nil,
			"clientSecret":   nil,
			"frontendDomain": nil,
			"keycloakDomain": nil,
			"allowedEmails":  []string{},
		},
		"generic": map[string]interface{}{
			"enabled":          false,
			"clientId":         nil,
			"clientSecret":     nil,
			"withPkce":         false,
			"authorizationUrl": nil,
			"tokenUrl":         nil,
			"frontendDomain":   nil,
			"allowedEmails":    []string{},
		},
		"telegram": map[string]interface{}{
			"enabled":        false,
			"clientId":       nil,
			"clientSecret":   nil,
			"allowedIds":     []string{},
			"frontendDomain": nil,
		},
	}
	if setting.OAuth2Settings != "" && setting.OAuth2Settings != "{}" {
		_ = json.Unmarshal([]byte(setting.OAuth2Settings), &oauth2Settings)
	}

	passwordSettings := map[string]interface{}{
		"enabled": setting.PasswordAuthEnabled,
	}
	if setting.PasswordSettings != "" && setting.PasswordSettings != "{}" {
		_ = json.Unmarshal([]byte(setting.PasswordSettings), &passwordSettings)
	}

	brandingSettings := map[string]interface{}{
		"title":   title,
		"logoUrl": nil,
	}
	if setting.LogoURL != "" {
		brandingSettings["logoUrl"] = setting.LogoURL
	}
	if setting.BrandingSettings != "" && setting.BrandingSettings != "{}" {
		_ = json.Unmarshal([]byte(setting.BrandingSettings), &brandingSettings)
	}

	resp := map[string]interface{}{
		"response": map[string]interface{}{
			"passkeySettings":  passkeySettings,
			"oauth2Settings":   oauth2Settings,
			"passwordSettings": passwordSettings,
			"brandingSettings": brandingSettings,
		},
	}
	json.NewEncoder(w).Encode(resp)
}

func (h *Handler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"message":"Invalid request body"}`, http.StatusBadRequest)
		return
	}

	var setting database.RemnawaveSetting
	h.db.First(&setting)
	if setting.ID == 0 {
		setting.ID = 1
		setting.Title = "Remnawave"
		setting.IsLoginAllowed = true
		setting.PasswordAuthEnabled = true
		h.db.Create(&setting)
	}

	if ps, ok := body["passkeySettings"]; ok {
		b, _ := json.Marshal(ps)
		setting.PasskeySettings = string(b)
	}
	if os, ok := body["oauth2Settings"]; ok {
		b, _ := json.Marshal(os)
		setting.OAuth2Settings = string(b)
	}
	if pws, ok := body["passwordSettings"]; ok {
		b, _ := json.Marshal(pws)
		setting.PasswordSettings = string(b)
		if m, ok := pws.(map[string]interface{}); ok {
			if en, ok := m["enabled"].(bool); ok {
				setting.PasswordAuthEnabled = en
			}
		}
	}
	if bs, ok := body["brandingSettings"]; ok {
		b, _ := json.Marshal(bs)
		setting.BrandingSettings = string(b)
		if m, ok := bs.(map[string]interface{}); ok {
			if t, ok := m["title"].(string); ok && t != "" {
				setting.Title = t
			}
			if lu, ok := m["logoUrl"].(string); ok {
				setting.LogoURL = lu
			}
		}
	}
	h.db.Save(&setting)
	h.GetSettings(w, r)
}

func (h *Handler) RestoreBackup(w http.ResponseWriter, r *http.Request) {
	file, _, err := r.FormFile("file")
	if err != nil {
		httpx.Error(w, r, http.StatusBadRequest, "E000", "No backup file uploaded")
		return
	}
	defer file.Close()

	out, err := os.CreateTemp("", "remnawave_restore_*.tar.gz")
	if err != nil {
		httpx.Error(w, r, http.StatusInternalServerError, httpx.CodeInternal, "Cannot create temporary file")
		return
	}
	tempPath := out.Name()
	defer os.Remove(tempPath)
	defer out.Close()

	if _, err := io.Copy(out, file); err != nil {
		httpx.Error(w, r, http.StatusInternalServerError, httpx.CodeInternal, "Failed saving backup")
		return
	}

	// Validation happens in a staging database; on failure the live data is
	// untouched.
	if err := h.restore.RestoreFromTarGz(tempPath); err != nil {
		httpx.Error(w, r, http.StatusBadRequest, "E000", "Backup restore failed: "+err.Error())
		return
	}
	if h.OnRestored != nil {
		h.OnRestored()
	}
	httpx.Response(w, map[string]string{"message": "Backup successfully restored"})
}
