package stubs

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"gowave/internal/httpx"

	"gorm.io/gorm"
)

// tableQuery is the upstream table query: start/size plus JSON-encoded
// filters, filterModes and sorting.
type tableQuery struct {
	Start   int
	Size    int
	Filters []struct {
		ID    string
		Value interface{}
	}
	FilterModes map[string]string
	Sorting     []struct {
		ID   string `json:"id"`
		Desc bool   `json:"desc"`
	}
}

func parseTableQuery(r *http.Request, defaultSize int) tableQuery {
	values := r.URL.Query()
	query := tableQuery{Size: defaultSize}
	query.Start, _ = strconv.Atoi(values.Get("start"))
	if size, err := strconv.Atoi(values.Get("size")); err == nil && size > 0 {
		query.Size = size
	}
	if query.Size > 1000 {
		query.Size = 1000
	}
	if query.Start < 0 {
		query.Start = 0
	}
	var filters []struct {
		ID    string      `json:"id"`
		Value interface{} `json:"value"`
	}
	_ = json.Unmarshal([]byte(values.Get("filters")), &filters)
	for _, filter := range filters {
		query.Filters = append(query.Filters, struct {
			ID    string
			Value interface{}
		}{filter.ID, filter.Value})
	}
	_ = json.Unmarshal([]byte(values.Get("filterModes")), &query.FilterModes)
	_ = json.Unmarshal([]byte(values.Get("sorting")), &query.Sorting)
	return query
}

// jsonText extracts a nested text value from a JSON column on both dialects.
func jsonText(db *gorm.DB, column string, path ...string) string {
	if db.Dialector.Name() == "postgres" {
		expression := "(" + column + "::jsonb)"
		for i, key := range path {
			operator := "->"
			if i == len(path)-1 {
				operator = "->>"
			}
			expression += operator + "'" + key + "'"
		}
		return expression
	}
	return "json_extract(" + column + ", '$." + strings.Join(path, ".") + "')"
}

func likeOperator(db *gorm.DB) string {
	if db.Dialector.Name() == "postgres" {
		return "ILIKE"
	}
	return "LIKE"
}

func (s *StubHandler) GetTorrentBlockerReports(w http.ResponseWriter, r *http.Request) {
	query := parseTableQuery(r, 25)
	filterColumns := map[string]string{
		"id":                            "torrent_blocker_reports.id",
		"userId":                        "torrent_blocker_reports.user_id",
		"nodeId":                        "torrent_blocker_reports.node_id",
		"createdAt":                     "torrent_blocker_reports.created_at",
		"user.username":                 "users.username",
		"node.uuid":                     "nodes.uuid",
		"node.name":                     "nodes.name",
		"report.actionReport.ip":        jsonText(s.db, "torrent_blocker_reports.report", "actionReport", "ip"),
		"report.xrayReport.inboundTag":  jsonText(s.db, "torrent_blocker_reports.report", "xrayReport", "inboundTag"),
		"report.xrayReport.outboundTag": jsonText(s.db, "torrent_blocker_reports.report", "xrayReport", "outboundTag"),
		"report.xrayReport.protocol":    jsonText(s.db, "torrent_blocker_reports.report", "xrayReport", "protocol"),
	}
	sortColumns := map[string]string{
		"userId":        "users.id",
		"user.username": "users.username",
		"node.uuid":     "nodes.uuid",
		"node.name":     "nodes.name",
		"id":            "torrent_blocker_reports.id",
		"createdAt":     "torrent_blocker_reports.created_at",
	}
	base := s.db.Table("torrent_blocker_reports").
		Joins("JOIN users ON users.id = torrent_blocker_reports.user_id").
		Joins("JOIN nodes ON nodes.id = torrent_blocker_reports.node_id")
	for _, filter := range query.Filters {
		column, ok := filterColumns[filter.ID]
		if !ok {
			continue
		}
		value := fmt.Sprint(filter.Value)
		switch {
		case filter.ID == "createdAt":
			at, err := time.Parse(time.RFC3339Nano, value)
			if err != nil {
				base = base.Where("1 = 0")
				continue
			}
			base = base.Where(column+" = ?", at.UTC())
		case filter.ID == "id" || filter.ID == "userId" || filter.ID == "nodeId":
			number, err := strconv.ParseUint(value, 10, 64)
			if err != nil {
				base = base.Where("1 = 0")
				continue
			}
			base = base.Where(column+" = ?", number)
		case filter.ID == "node.uuid":
			base = base.Where("CAST("+column+" AS TEXT) "+likeOperator(s.db)+" ?", "%"+value+"%")
		case query.FilterModes[filter.ID] == "equals":
			base = base.Where(column+" = ?", value)
		default:
			base = base.Where(column+" "+likeOperator(s.db)+" ?", "%"+value+"%")
		}
	}
	var total int64
	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		httpx.Error(w, r, http.StatusInternalServerError, "A227", "Get torrent blocker reports error")
		return
	}
	listing := base.Select("torrent_blocker_reports.id, torrent_blocker_reports.user_id, torrent_blocker_reports.node_id, torrent_blocker_reports.report, torrent_blocker_reports.created_at, users.username, nodes.uuid AS node_uuid, nodes.name AS node_name, nodes.country_code")
	if len(query.Sorting) == 0 {
		listing = listing.Order("torrent_blocker_reports.id DESC")
	}
	for _, sort := range query.Sorting {
		column, ok := sortColumns[sort.ID]
		if !ok {
			continue
		}
		direction := " ASC"
		if sort.Desc {
			direction = " DESC"
		}
		listing = listing.Order(column + direction)
	}
	var rows []struct {
		ID          uint64
		UserID      uint64
		NodeID      uint64
		Report      string
		CreatedAt   time.Time
		Username    string
		NodeUUID    string
		NodeName    string
		CountryCode string
	}
	if err := listing.Offset(query.Start).Limit(query.Size).Scan(&rows).Error; err != nil {
		httpx.Error(w, r, http.StatusInternalServerError, "A227", "Get torrent blocker reports error")
		return
	}
	records := make([]map[string]interface{}, 0, len(rows))
	for _, row := range rows {
		var report interface{}
		if err := json.Unmarshal([]byte(row.Report), &report); err != nil {
			report = map[string]interface{}{}
		}
		records = append(records, map[string]interface{}{
			"id":        row.ID,
			"userId":    row.UserID,
			"nodeId":    row.NodeID,
			"user":      map[string]string{"username": row.Username},
			"node":      map[string]string{"uuid": row.NodeUUID, "name": row.NodeName, "countryCode": row.CountryCode},
			"report":    report,
			"createdAt": row.CreatedAt.UTC().Format(time.RFC3339Nano),
		})
	}
	httpx.Response(w, map[string]interface{}{"records": records, "total": total})
}

func (s *StubHandler) GetTorrentBlockerStats(w http.ResponseWriter, r *http.Request) {
	var stats struct {
		TotalReports       int64
		DistinctUsers      int64
		DistinctNodes      int64
		ReportsLast24Hours int64
	}
	err := s.db.Table("torrent_blocker_reports").Select(
		"COUNT(*) AS total_reports, COUNT(DISTINCT user_id) AS distinct_users, COUNT(DISTINCT node_id) AS distinct_nodes, "+
			"COALESCE(SUM(CASE WHEN created_at > ? THEN 1 ELSE 0 END), 0) AS reports_last24_hours",
		time.Now().UTC().Add(-24*time.Hour),
	).Scan(&stats).Error
	var topUsers []struct {
		UserID   uint64
		Username string
		Total    int64
	}
	if err == nil {
		err = s.db.Table("torrent_blocker_reports").Joins("JOIN users ON users.id = torrent_blocker_reports.user_id").
			Select("users.id AS user_id, users.username, COUNT(*) AS total").Group("users.id, users.username").
			Order("total DESC").Limit(150).Scan(&topUsers).Error
	}
	var topNodes []struct {
		UUID        string
		Name        string
		CountryCode string
		Total       int64
	}
	if err == nil {
		err = s.db.Table("torrent_blocker_reports").Joins("JOIN nodes ON nodes.id = torrent_blocker_reports.node_id").
			Select("nodes.uuid, nodes.name, nodes.country_code, COUNT(*) AS total").Group("nodes.uuid, nodes.name, nodes.country_code").
			Order("total DESC").Limit(150).Scan(&topNodes).Error
	}
	if err != nil {
		httpx.Error(w, r, http.StatusInternalServerError, httpx.CodeInternal, "Server error")
		return
	}
	users := make([]map[string]interface{}, 0, len(topUsers))
	for _, user := range topUsers {
		users = append(users, map[string]interface{}{"userId": user.UserID, "color": httpx.ColorFromID(user.UserID), "username": user.Username, "total": user.Total})
	}
	nodes := make([]map[string]interface{}, 0, len(topNodes))
	for _, node := range topNodes {
		nodes = append(nodes, map[string]interface{}{"uuid": node.UUID, "color": httpx.ColorFromUUID(node.UUID), "name": node.Name, "countryCode": node.CountryCode, "total": node.Total})
	}
	httpx.Response(w, map[string]interface{}{
		"stats": map[string]int64{
			"distinctNodes":      stats.DistinctNodes,
			"distinctUsers":      stats.DistinctUsers,
			"totalReports":       stats.TotalReports,
			"reportsLast24Hours": stats.ReportsLast24Hours,
		},
		"topUsers": users,
		"topNodes": nodes,
	})
}

func (s *StubHandler) TruncateTorrentBlockerReports(w http.ResponseWriter, r *http.Request) {
	statement := "DELETE FROM torrent_blocker_reports"
	if s.db.Dialector.Name() == "postgres" {
		statement = "TRUNCATE torrent_blocker_reports RESTART IDENTITY"
	}
	if err := s.db.Exec(statement).Error; err != nil {
		httpx.Error(w, r, http.StatusInternalServerError, httpx.CodeInternal, "Server error")
		return
	}
	if s.db.Dialector.Name() == "sqlite" {
		s.db.Exec("DELETE FROM sqlite_sequence WHERE name = 'torrent_blocker_reports'")
	}
	w.WriteHeader(http.StatusNoContent)
}
