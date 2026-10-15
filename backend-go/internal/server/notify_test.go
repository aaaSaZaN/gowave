package server

import (
	"encoding/json"
	"sort"
	"strings"
	"testing"
	"time"

	"remnawave-go/config"
	"remnawave-go/internal/database"
	"remnawave-go/internal/notify"
)

// TestActionsEnqueueNotifications checks that API actions write the upstream
// webhook events to the outbox in the same transaction as the change.
func TestActionsEnqueueNotifications(t *testing.T) {
	e := newProbeEnv(t)
	notifier, err := notify.New(&config.Config{Notifications: config.NotificationsConfig{
		WebhookEnabled: true, WebhookURLs: []string{"https://hooks.example.com/rw"}, WebhookSecretHeader: strings.Repeat("x", 32),
	}})
	if err != nil {
		t.Fatal(err)
	}
	notify.SetDefault(notifier)
	t.Cleanup(func() { notify.SetDefault(nil) })

	expire := time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339)
	status, data := e.do("POST", "/api/users", map[string]interface{}{"username": "events_user", "expireAt": expire})
	if status != 201 {
		t.Fatalf("create: %d %s", status, data)
	}
	id := responseField(data, "id")
	for _, step := range []struct{ method, path string }{
		{"POST", "/api/users/" + id + "/actions/disable"},
		{"POST", "/api/users/" + id + "/actions/enable"},
		{"POST", "/api/users/" + id + "/actions/reset-traffic"},
		{"POST", "/api/users/" + id + "/actions/revoke"},
		{"DELETE", "/api/users/" + id},
	} {
		if status, data := e.do(step.method, step.path, map[string]interface{}{}); status >= 300 {
			t.Fatalf("%s %s: %d %s", step.method, step.path, status, data)
		}
	}
	// A rejected action must not notify.
	e.do("POST", "/api/users/"+id+"/actions/enable", nil)

	var jobs []database.Job
	e.db.Where("kind = ?", "webhook").Order("created_at, id").Find(&jobs)
	var events []string
	for _, job := range jobs {
		var payload struct {
			Payload string `json:"payload"`
		}
		json.Unmarshal([]byte(job.Payload), &payload)
		var body struct {
			Event string `json:"event"`
			Scope string `json:"scope"`
			Data  struct {
				Username string `json:"username"`
			} `json:"data"`
		}
		json.Unmarshal([]byte(payload.Payload), &body)
		if body.Scope != "user" || body.Data.Username != "events_user" {
			t.Fatalf("unexpected payload %s", payload.Payload)
		}
		events = append(events, body.Event)
	}
	sort.Strings(events)
	want := []string{"user.created", "user.deleted", "user.disabled", "user.enabled", "user.revoked", "user.traffic_reset"}
	if strings.Join(events, ",") != strings.Join(want, ",") {
		t.Fatalf("events = %v, want %v", events, want)
	}
}
