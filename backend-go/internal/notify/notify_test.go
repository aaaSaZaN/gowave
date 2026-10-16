package notify

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gowave/config"
	"gowave/internal/database"
	"gowave/internal/jobs"
	"gowave/internal/testdb"

	"gorm.io/gorm"
)

// Vectors produced by xbytes 1.9.1 through upstream prettyBytesUtil.
func TestPrettyBytesMatchesUpstream(t *testing.T) {
	cases := []struct {
		bytes  float64
		sticky bool
		want   string
	}{
		{1, false, "1.00 B"}, {1023, false, "1023.00 B"}, {1024, false, "1.00 KiB"}, {1536, false, "1.50 KiB"},
		{1073741824, false, "1.00 GiB"}, {5e12, false, "4.55 TiB"}, {123456789, false, "117.74 MiB"},
		{1, true, "0.00 GiB"}, {1610612736, true, "1.50 GiB"}, {5e12, true, "4656.61 GiB"}, {0, false, "0"},
	}
	for _, c := range cases {
		if got := prettyBytes(c.bytes, c.sticky, 3); got != c.want {
			t.Errorf("prettyBytes(%v, %v) = %q, want %q", c.bytes, c.sticky, got, c.want)
		}
	}
}

func testDB(t *testing.T) *gorm.DB { return testdb.Open(t) }

func TestWebhookDeliverySignsPayloadAndHonoursEventConfig(t *testing.T) {
	secret := strings.Repeat("a", 32)
	received := make(chan *http.Request, 4)
	bodies := make(chan string, 4)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		received <- r
		bodies <- string(data)
	}))
	defer server.Close()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "notifications.yml")
	os.WriteFile(configPath, []byte("events:\n  user.deleted:\n    telegram: false\n    webhook: false\n"), 0o600)
	t.Setenv("NOTIFICATIONS_CONFIG_PATH", configPath)

	cfg := &config.Config{Notifications: config.NotificationsConfig{WebhookEnabled: true, WebhookURLs: []string{server.URL}, WebhookSecretHeader: secret}}
	notifier, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	db := testDB(t)
	queue := jobs.New(db)
	notifier.Register(queue)

	user := map[string]interface{}{"id": 7, "username": "alice"}
	if err := db.Transaction(func(tx *gorm.DB) error {
		return notifier.Emit(tx, Event{Name: "user.created", Data: user}, Event{Name: "user.deleted", Data: user})
	}); err != nil {
		t.Fatal(err)
	}
	// A rolled-back change must not notify.
	_ = db.Transaction(func(tx *gorm.DB) error {
		notifier.Emit(tx, Event{Name: "user.modified", Data: user})
		return errors.New("rollback")
	})
	var count int64
	db.Model(&database.Job{}).Where("kind = ?", "webhook").Count(&count)
	if count != 1 {
		t.Fatalf("expected 1 webhook job (deleted disabled, modified rolled back), got %d", count)
	}
	if worked, err := queue.RunOne(context.Background()); !worked || err != nil {
		t.Fatalf("RunOne: %v %v", worked, err)
	}
	req, body := <-received, <-bodies
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(body))
	if req.Header.Get("X-Remnawave-Signature") != hex.EncodeToString(mac.Sum(nil)) {
		t.Fatal("signature does not match HMAC-SHA256 of the body")
	}
	var payload map[string]interface{}
	json.Unmarshal([]byte(body), &payload)
	if payload["scope"] != "user" || payload["event"] != "user.created" || req.Header.Get("X-Remnawave-Timestamp") != payload["timestamp"] {
		t.Fatalf("unexpected payload %s", body)
	}
}

func TestTelegramTemplatesEscapeValues(t *testing.T) {
	n := &Notifier{}
	message, _ := n.render(Event{Name: "service.login_attempt_failed", Data: map[string]interface{}{
		"loginAttempt": map[string]string{"username": "<script>", "password": "a&b", "ip": "1.2.3.4", "userAgent": "x", "description": "bad"},
	}})
	if !strings.Contains(message, "&lt;script&gt;") || !strings.Contains(message, "a&amp;b") || strings.Contains(message, "<script>") {
		t.Fatalf("values were not escaped: %s", message)
	}
	message, _ = n.render(Event{Name: "user.created", Data: map[string]interface{}{
		"username": "bob", "trafficLimitBytes": 1073741824, "expireAt": "2026-03-04T05:06:00.000Z", "shortUuid": "abc",
		"activeInternalSquads": []map[string]string{{"name": "Default"}},
	}})
	for _, want := range []string{"#created", "<code>bob</code>", "1.00 GiB", "04.03.2026 05:06 UTC", "Default"} {
		if !strings.Contains(message, want) {
			t.Fatalf("user.created message lacks %q:\n%s", want, message)
		}
	}
	if message, _ := n.render(Event{Name: "user.expiration", Data: map[string]string{"username": "bob"}, Meta: map[string]int{"expiration": -24}}); !strings.Contains(message, "#expires_in_24_hours") {
		t.Fatalf("expiration tag: %s", message)
	}
}

func TestUnknownEventInConfigIsRejected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "n.yml")
	os.WriteFile(path, []byte("events:\n  user.unknown:\n    telegram: true\n    webhook: true\n"), 0o600)
	t.Setenv("NOTIFICATIONS_CONFIG_PATH", path)
	if _, err := New(&config.Config{}); err == nil {
		t.Fatal("expected unknown event to fail validation")
	}
}
