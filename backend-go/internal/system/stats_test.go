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

	"gowave/internal/database"
	"gowave/internal/testdb"
)

func TestGetStatsCountsUsersByStatus(t *testing.T) {
	db := testdb.Open(t)
	statuses := []string{"ACTIVE", "ACTIVE", "ACTIVE", "DISABLED", "EXPIRED"}
	for i, status := range statuses {
		name := fmt.Sprintf("stats-user-%d", i)
		user := database.User{Username: name, ShortUUID: name, VlessUUID: name, Status: status}
		if err := db.Create(&user).Error; err != nil {
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
	if got, want := DockerUpdateCommand(), "cd /opt/gowave-docker && docker compose pull && docker compose up -d"; got != want {
		t.Errorf("DockerUpdateCommand() = %q, want %q", got, want)
	}

	t.Setenv("IN_DOCKER", "true")
	recorder := httptest.NewRecorder()
	(&Handler{}).ApplyUpdate(recorder, httptest.NewRequest(http.MethodPost, "/api/system/updates/apply", nil))
	if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), "docker compose pull") {
		t.Errorf("ApplyUpdate in Docker = %d %q, want 400 with the update command", recorder.Code, recorder.Body.String())
	}
}
