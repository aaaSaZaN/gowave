package system

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gowave/internal/database"
	"gowave/internal/testdb"
)

func TestGetStatsCountsUsersByStatus(t *testing.T) {
	db := testdb.Open(t)
	statuses := []string{"ACTIVE", "ACTIVE", "ACTIVE", "DISABLED", "EXPIRED"}
	now := time.Now().UTC()
	// User 0 is online now, user 1 was online two days ago, user 2 has a
	// traffic row but never connected, users 3-4 have no traffic row.
	onlineAt := []*time.Time{ptrTime(now.Add(-5 * time.Second)), ptrTime(now.Add(-48 * time.Hour)), nil}
	for i, status := range statuses {
		name := fmt.Sprintf("stats-user-%d", i)
		user := database.User{Username: name, ShortUUID: name, VlessUUID: name, Status: status}
		if err := db.Create(&user).Error; err != nil {
			t.Fatal(err)
		}
		if i < len(onlineAt) {
			if err := db.Create(&database.UserTraffic{ID: user.ID, OnlineAt: onlineAt[i]}).Error; err != nil {
				t.Fatal(err)
			}
		}
	}
	nodes := []database.Node{
		{UUID: "node-online", Name: "online", Address: "10.0.0.1", IsConnected: true},
		{UUID: "node-disabled", Name: "disabled", Address: "10.0.0.2", IsConnected: true, IsDisabled: true},
		{UUID: "node-offline", Name: "offline", Address: "10.0.0.3"},
	}
	for i := range nodes {
		if err := db.Create(&nodes[i]).Error; err != nil {
			t.Fatal(err)
		}
	}
	for i, bytes := range []uint64{1000, 2500} {
		if err := db.Create(&database.NodesUsageHistory{NodeUUID: "node-online", CreatedAt: now.Add(-time.Duration(i+1) * time.Hour), TotalBytes: bytes}).Error; err != nil {
			t.Fatal(err)
		}
	}

	recorder := httptest.NewRecorder()
	NewHandler(db).GetStats(recorder, httptest.NewRequest(http.MethodGet, "/api/system/stats", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("GET /api/system/stats = %d: %s", recorder.Code, recorder.Body.String())
	}
	var resp SystemStatsResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}

	users := resp.Response.Users
	if users.TotalUsers != len(statuses) {
		t.Errorf("totalUsers = %d, want %d", users.TotalUsers, len(statuses))
	}
	want := map[string]int{"ACTIVE": 3, "DISABLED": 1, "LIMITED": 0, "EXPIRED": 1}
	for status, count := range want {
		if got, ok := users.StatusCounts[status]; !ok || got != count {
			t.Errorf("statusCounts[%s] = %d (present %v), want %d", status, got, ok, count)
		}
	}
	wantOnline := map[string]int{"onlineNow": 1, "lastDay": 1, "lastWeek": 2, "neverOnline": 3}
	gotOnline := map[string]int{"onlineNow": resp.Response.OnlineStats.OnlineNow, "lastDay": resp.Response.OnlineStats.LastDay,
		"lastWeek": resp.Response.OnlineStats.LastWeek, "neverOnline": resp.Response.OnlineStats.NeverOnline}
	for key, want := range wantOnline {
		if gotOnline[key] != want {
			t.Errorf("onlineStats.%s = %d, want %d", key, gotOnline[key], want)
		}
	}
	if got := resp.Response.Nodes.TotalOnline; got != 1 {
		t.Errorf("nodes.totalOnline = %d, want 1 (connected and enabled)", got)
	}
	if got := resp.Response.Nodes.TotalBytesLifetime; got != "3500" {
		t.Errorf("nodes.totalBytesLifetime = %s, want 3500", got)
	}
	memory := resp.Response.Memory
	if memory.Total == 0 || memory.Used+memory.Free != memory.Total {
		t.Errorf("memory = %+v, want used + free = total > 0", memory)
	}
}

func TestReadProcMeminfo(t *testing.T) {
	path := filepath.Join(t.TempDir(), "meminfo")
	content := "MemTotal:       16318412 kB\nMemFree:         1048576 kB\nMemAvailable:    8000000 kB\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	total, free, ok := readProcMeminfo(path)
	if !ok || total != 16318412*1024 || free != 1048576*1024 {
		t.Errorf("readProcMeminfo = %d, %d, %v", total, free, ok)
	}
	if _, _, ok := readProcMeminfo(filepath.Join(t.TempDir(), "missing")); ok {
		t.Error("missing file must not be reported as readable")
	}
}

func TestDockerUpdateCommandUsesComposeDir(t *testing.T) {
	t.Setenv("GOWAVE_COMPOSE_DIR", "/opt/gowave-docker")
	if got, want := DockerUpdateCommand(CurrentChannel()), "cd /opt/gowave-docker && docker compose pull && docker compose up -d"; got != want {
		t.Errorf("DockerUpdateCommand() = %q, want %q", got, want)
	}

	t.Setenv("IN_DOCKER", "true")
	recorder := httptest.NewRecorder()
	(&Handler{}).ApplyUpdate(recorder, httptest.NewRequest(http.MethodPost, "/api/system/updates/apply", nil))
	if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), "docker compose pull") {
		t.Errorf("ApplyUpdate in Docker = %d %q, want 400 with the update command", recorder.Code, recorder.Body.String())
	}
}

func ptrTime(t time.Time) *time.Time { return &t }

func TestGetStatsConcurrentRequestsShareResult(t *testing.T) {
	db := testdb.Open(t)
	for i := 0; i < 3; i++ {
		name := fmt.Sprintf("concurrent-%d", i)
		if err := db.Create(&database.User{Username: name, ShortUUID: name, VlessUUID: name, Status: "ACTIVE"}).Error; err != nil {
			t.Fatal(err)
		}
	}
	handler := NewHandler(db)
	results := make(chan SystemStatsResponse, 16)
	for i := 0; i < cap(results); i++ {
		go func() {
			recorder := httptest.NewRecorder()
			handler.GetStats(recorder, httptest.NewRequest(http.MethodGet, "/api/system/stats", nil))
			var resp SystemStatsResponse
			if recorder.Code != http.StatusOK || json.Unmarshal(recorder.Body.Bytes(), &resp) != nil {
				t.Errorf("GET /api/system/stats = %d: %s", recorder.Code, recorder.Body.String())
			}
			results <- resp
		}()
	}
	for i := 0; i < cap(results); i++ {
		if resp := <-results; resp.Response.Users.TotalUsers != 3 || resp.Response.Users.StatusCounts["ACTIVE"] != 3 {
			t.Errorf("concurrent response users = %+v", resp.Response.Users)
		}
	}
}
