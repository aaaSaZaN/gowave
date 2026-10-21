package nodes

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gowave/internal/database"
	"gowave/internal/testdb"

	"gorm.io/gorm"
)

func TestTrafficPeriodStart(t *testing.T) {
	at := func(y int, m time.Month, d, h int) time.Time { return time.Date(y, m, d, h, 0, 0, 0, time.UTC) }
	for _, tc := range []struct {
		now      time.Time
		resetDay int
		want     time.Time
	}{
		{at(2026, 10, 21, 12), 1, at(2026, 10, 1, 1)},
		{at(2026, 10, 21, 12), 25, at(2026, 9, 25, 1)},
		{at(2026, 10, 21, 12), 31, at(2026, 9, 30, 1)},
		{at(2026, 3, 1, 0), 1, at(2026, 2, 1, 1)},
		{at(2026, 2, 28, 5), 31, at(2026, 2, 28, 1)},
		{at(2026, 1, 10, 5), 15, at(2025, 12, 15, 1)},
		{at(2026, 10, 21, 12), 0, at(2026, 10, 1, 1)},
	} {
		if got := trafficPeriodStart(tc.now, tc.resetDay); !got.Equal(tc.want) {
			t.Errorf("trafficPeriodStart(%s, %d) = %s, want %s", tc.now, tc.resetDay, got, tc.want)
		}
	}
}

func createUsage(t *testing.T, db *gorm.DB, rows ...database.NodesUsageHistory) {
	t.Helper()
	for _, row := range rows {
		if err := db.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
}

func TestApplyTrackingStart(t *testing.T) {
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	for mode, want := range map[string]uint64{"": 500, TrackingStartKeep: 500, TrackingStartZero: 0, TrackingStartPeriod: 300} {
		t.Run("mode="+mode, func(t *testing.T) {
			db := testdb.Open(t)
			node := database.Node{UUID: "node", Name: "node", Address: "example.com", TrafficUsedBytes: 500, IsTrafficTrackingActive: true, TrafficResetDay: 10, NodeConsumptionMultiplier: 2_000_000_000}
			if err := db.Create(&node).Error; err != nil {
				t.Fatal(err)
			}
			createUsage(t, db,
				// Before the 01:00 reset on the 10th: the previous period.
				database.NodesUsageHistory{NodeUUID: "node", CreatedAt: time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC), TotalBytes: 1000},
				database.NodesUsageHistory{NodeUUID: "node", CreatedAt: time.Date(2026, 10, 10, 1, 0, 0, 0, time.UTC), TotalBytes: 100},
				database.NodesUsageHistory{NodeUUID: "node", CreatedAt: time.Date(2026, 10, 20, 15, 0, 0, 0, time.UTC), TotalBytes: 50},
			)
			if err := applyTrackingStart(db, node.UUID, mode, now); err != nil {
				t.Fatal(err)
			}
			if err := db.Where("uuid = ?", node.UUID).First(&node).Error; err != nil {
				t.Fatal(err)
			}
			if node.TrafficUsedBytes != want {
				t.Fatalf("trafficUsedBytes = %d, want %d", node.TrafficUsedBytes, want)
			}
		})
	}
}

func patchNode(t *testing.T, handler *Handler, body string) *httptest.ResponseRecorder {
	t.Helper()
	recorder := httptest.NewRecorder()
	handler.PatchNode(recorder, httptest.NewRequest(http.MethodPatch, "/api/nodes", strings.NewReader(body)))
	return recorder
}

func TestUpdateNodeTrafficTrackingStart(t *testing.T) {
	db := testdb.Open(t)
	node := database.Node{UUID: "node", Name: "node", Address: "example.com", TrafficUsedBytes: 500, TrafficResetDay: 1}
	if err := db.Create(&node).Error; err != nil {
		t.Fatal(err)
	}
	periodStart := trafficPeriodStart(time.Now(), 1)
	createUsage(t, db,
		database.NodesUsageHistory{NodeUUID: "node", CreatedAt: periodStart.Add(-time.Hour), TotalBytes: 1000},
		database.NodesUsageHistory{NodeUUID: "node", CreatedAt: periodStart, TotalBytes: 70},
	)
	handler := NewHandler(NewService(db, nil))

	if recorder := patchNode(t, handler, `{"uuid":"node","isTrafficTrackingActive":true,"trafficTrackingStart":"bogus"}`); recorder.Code != http.StatusBadRequest {
		t.Fatalf("invalid trafficTrackingStart = %d: %s", recorder.Code, recorder.Body.String())
	}

	recorder := patchNode(t, handler, `{"uuid":"node","isTrafficTrackingActive":true,"trafficTrackingStart":"period"}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("PATCH /api/nodes = %d: %s", recorder.Code, recorder.Body.String())
	}
	var res struct {
		Response struct {
			TrafficUsedBytes        uint64 `json:"trafficUsedBytes"`
			IsTrafficTrackingActive bool   `json:"isTrafficTrackingActive"`
		} `json:"response"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if !res.Response.IsTrafficTrackingActive || res.Response.TrafficUsedBytes != 70 {
		t.Fatalf("response = %+v, want tracking with 70 bytes", res.Response)
	}

	// Only enabling tracking moves the counter.
	if recorder := patchNode(t, handler, `{"uuid":"node","isTrafficTrackingActive":true,"trafficTrackingStart":"zero"}`); recorder.Code != http.StatusOK {
		t.Fatalf("PATCH /api/nodes = %d: %s", recorder.Code, recorder.Body.String())
	}
	if err := db.Where("uuid = ?", node.UUID).First(&node).Error; err != nil {
		t.Fatal(err)
	}
	if node.TrafficUsedBytes != 70 {
		t.Fatalf("already tracked node counter changed to %d", node.TrafficUsedBytes)
	}
}

func TestGetTrafficSummary(t *testing.T) {
	db := testdb.Open(t)
	for _, record := range []interface{}{
		&database.Node{UUID: "a", Name: "a", Address: "10.0.0.1", TrafficUsedBytes: 1},
		&database.Node{UUID: "b", Name: "b", Address: "10.0.0.2"},
		&database.NodesUsageHistory{NodeUUID: "a", CreatedAt: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), TotalBytes: 40},
		&database.NodesUsageHistory{NodeUUID: "a", CreatedAt: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), TotalBytes: 2},
	} {
		if err := db.Create(record).Error; err != nil {
			t.Fatal(err)
		}
	}
	recorder := httptest.NewRecorder()
	NewHandler(NewService(db, nil)).GetTrafficSummary(recorder, httptest.NewRequest(http.MethodGet, "/api/nodes/traffic-summary", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("GET /api/nodes/traffic-summary = %d: %s", recorder.Code, recorder.Body.String())
	}
	var res struct {
		Response struct {
			Nodes []struct {
				UUID                 string `json:"uuid"`
				LifetimeTrafficBytes int64  `json:"lifetimeTrafficBytes"`
			} `json:"nodes"`
		} `json:"response"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if len(res.Response.Nodes) != 1 || res.Response.Nodes[0].UUID != "a" || res.Response.Nodes[0].LifetimeTrafficBytes != 42 {
		t.Fatalf("summary = %+v, want node a with 42 bytes", res.Response.Nodes)
	}
}
