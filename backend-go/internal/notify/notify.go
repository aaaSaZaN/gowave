// Package notify delivers upstream-compatible webhook and Telegram
// notifications. Events are written to the job outbox in the caller's
// transaction, so a rolled-back change never notifies and a committed one is
// retried until delivered.
package notify

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"gowave/config"
	"gowave/internal/database"
	"gowave/internal/jobs"

	"gopkg.in/yaml.v3"
	"gorm.io/gorm"
)

// Event is one upstream notification, e.g. Name "user.created".
type Event struct {
	Name string
	// Data is the webhook "data" field, already in API response shape.
	Data interface{}
	// Meta is the optional webhook "meta" field (user events).
	Meta interface{}
	// SkipTelegram suppresses the Telegram message for this occurrence.
	SkipTelegram bool
}

// Scope is the part of the event name before the first dot.
func (e Event) Scope() string { return strings.SplitN(e.Name, ".", 2)[0] }

type eventConfig struct {
	Telegram              bool     `yaml:"telegram"`
	Webhook               bool     `yaml:"webhook"`
	AdditionalWebhookURLs []string `yaml:"additionalWebhookUrls"`
}

// Notifier renders events into delivery jobs.
type Notifier struct {
	cfg         config.NotificationsConfig
	panelDomain string
	events      map[string]eventConfig
	client      *http.Client
	telegram    *http.Client
}

// ConfigPaths are searched in order for the per-event YAML file used by
// upstream (configs/notifications/notifications-config.yml).
var ConfigPaths = []string{
	"/var/lib/remnawave/configs/notifications/notifications-config.yml",
	"configs/notifications/notifications-config.yml",
}

// New loads per-event settings from NOTIFICATIONS_CONFIG_PATH or ConfigPaths.
// A missing file enables every event on every configured channel.
func New(cfg *config.Config) (*Notifier, error) {
	n := &Notifier{
		cfg:         cfg.Notifications,
		panelDomain: cfg.PanelDomain,
		events:      map[string]eventConfig{},
		client:      &http.Client{Timeout: 15 * time.Second},
		telegram:    &http.Client{Timeout: 15 * time.Second},
	}
	if proxy := cfg.Notifications.TelegramProxy; proxy != "" {
		proxyURL, err := url.Parse(proxy)
		if err != nil {
			return nil, fmt.Errorf("TELEGRAM_BOT_PROXY: %w", err)
		}
		n.telegram.Transport = &http.Transport{Proxy: http.ProxyURL(proxyURL)}
	}
	paths := ConfigPaths
	if custom := os.Getenv("NOTIFICATIONS_CONFIG_PATH"); custom != "" {
		paths = []string{custom}
	}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		var file struct {
			Events map[string]eventConfig `yaml:"events"`
		}
		if err := yaml.Unmarshal(data, &file); err != nil {
			return nil, fmt.Errorf("notifications config %s: %w", path, err)
		}
		for name := range file.Events {
			if !knownEvent(name) {
				return nil, fmt.Errorf("notifications config %s: unknown event %q", path, name)
			}
		}
		if file.Events != nil {
			n.events = file.Events
		}
		break
	}
	return n, nil
}

func (n *Notifier) enabled(name, channel string) bool {
	event, ok := n.events[name]
	if !ok {
		return true
	}
	if channel == "telegram" {
		return event.Telegram
	}
	return event.Webhook
}

// telegramTarget returns the configured "chatId[:threadId]" for a scope.
func (n *Notifier) telegramTarget(scope string) string {
	switch scope {
	case "user":
		return n.cfg.TelegramUsers
	case "node":
		return n.cfg.TelegramNodes
	case "service", "errors":
		return n.cfg.TelegramService
	case "crm":
		return n.cfg.TelegramCRM
	case "torrent_blocker":
		return n.cfg.TelegramTBlocker
	}
	return ""
}

type webhookJob struct {
	URL       string `json:"url"`
	Payload   string `json:"payload"`
	Timestamp string `json:"timestamp"`
}

type telegramJob struct {
	ChatID   string           `json:"chatId"`
	ThreadID string           `json:"threadId,omitempty"`
	Message  string           `json:"message"`
	Keyboard []inlineKeyboard `json:"keyboard,omitempty"`
}

// uniqueRevision keeps outbox ids distinct for repeated identical events.
func uniqueRevision() uint64 {
	var random [8]byte
	_, _ = rand.Read(random[:])
	return uint64(time.Now().UnixNano()) ^ binary.LittleEndian.Uint64(random[:])>>16
}

// Emit enqueues webhook and Telegram deliveries for events inside tx.
func (n *Notifier) Emit(tx *gorm.DB, events ...Event) error {
	for _, event := range events {
		timestamp := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
		if n.cfg.WebhookEnabled && n.enabled(event.Name, "webhook") {
			payload := map[string]interface{}{
				"scope":     event.Scope(),
				"event":     event.Name,
				"timestamp": timestamp,
				"data":      event.Data,
			}
			if event.Meta != nil {
				payload["meta"] = event.Meta
			}
			body, err := json.Marshal(payload)
			if err != nil {
				return err
			}
			urls := append(append([]string{}, n.cfg.WebhookURLs...), n.events[event.Name].AdditionalWebhookURLs...)
			for _, target := range urls {
				job := webhookJob{URL: target, Payload: string(body), Timestamp: timestamp}
				// Deliveries to one endpoint are serialized and keep event order.
				digest := sha256.Sum256([]byte(target))
				if err := jobs.Enqueue(tx, "webhook", event.Name, "webhook:"+hex.EncodeToString(digest[:16]), uniqueRevision(), job); err != nil {
					return err
				}
			}
		}
		target := n.telegramTarget(event.Scope())
		if !n.cfg.TelegramEnabled || target == "" || event.SkipTelegram || !n.enabled(event.Name, "telegram") {
			continue
		}
		message, keyboard := n.render(event)
		if message == "" {
			continue
		}
		chat, thread, _ := strings.Cut(target, ":")
		job := telegramJob{ChatID: chat, ThreadID: thread, Message: message, Keyboard: keyboard}
		if err := jobs.Enqueue(tx, "telegram", event.Name, "telegram", uniqueRevision(), job); err != nil {
			return err
		}
	}
	return nil
}

// Register attaches the delivery handlers to the worker queue.
func (n *Notifier) Register(queue *jobs.Queue) {
	queue.Register("webhook", n.deliverWebhook)
	queue.Register("telegram", n.deliverTelegram)
}

func (n *Notifier) deliverWebhook(ctx context.Context, job *database.Job) error {
	var payload webhookJob
	if err := json.Unmarshal([]byte(job.Payload), &payload); err != nil {
		return err
	}
	if n.cfg.WebhookSecretHeader == "" {
		return fmt.Errorf("WEBHOOK_SECRET_HEADER is not set")
	}
	mac := hmac.New(sha256.New, []byte(n.cfg.WebhookSecretHeader))
	mac.Write([]byte(payload.Payload))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, payload.URL, strings.NewReader(payload.Payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Remnawave-Signature", hex.EncodeToString(mac.Sum(nil)))
	req.Header.Set("X-Remnawave-Timestamp", payload.Timestamp)
	req.Header.Set("User-Agent", "Remnawave")
	res, err := n.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(res.Body, 64<<10))
	if res.StatusCode >= 300 {
		return fmt.Errorf("webhook %s answered %d", payload.URL, res.StatusCode)
	}
	return nil
}

func (n *Notifier) deliverTelegram(ctx context.Context, job *database.Job) error {
	var payload telegramJob
	if err := json.Unmarshal([]byte(job.Payload), &payload); err != nil {
		return err
	}
	body := map[string]interface{}{
		"chat_id":              payload.ChatID,
		"text":                 payload.Message,
		"parse_mode":           "HTML",
		"link_preview_options": map[string]bool{"is_disabled": true},
	}
	if thread, err := strconv.Atoi(payload.ThreadID); err == nil && thread > 0 {
		body["message_thread_id"] = thread
	}
	if len(payload.Keyboard) > 0 {
		rows := make([][]map[string]string, 0, len(payload.Keyboard))
		for _, button := range payload.Keyboard {
			entry := map[string]string{"text": button.Text, "url": button.URL}
			if button.CustomEmoji != "" {
				entry["icon_custom_emoji_id"] = button.CustomEmoji
			}
			if button.Style != "" {
				entry["style"] = button.Style
			}
			rows = append(rows, []map[string]string{entry})
		}
		body["reply_markup"] = map[string]interface{}{"inline_keyboard": rows}
	}
	data, _ := json.Marshal(body)
	endpoint := n.cfg.TelegramAPIRoot + "/bot" + n.cfg.TelegramBotToken + "/sendMessage"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := n.telegram.Do(req)
	if err != nil {
		// Never log the bot token embedded in the URL.
		return fmt.Errorf("telegram network error")
	}
	defer res.Body.Close()
	var reply struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
		Parameters  struct {
			RetryAfter int `json:"retry_after"`
		} `json:"parameters"`
	}
	_ = json.NewDecoder(io.LimitReader(res.Body, 64<<10)).Decode(&reply)
	if res.StatusCode == http.StatusTooManyRequests && reply.Parameters.RetryAfter > 0 {
		return &jobs.RetryAfterError{Err: fmt.Errorf("telegram rate limited"), Delay: time.Duration(reply.Parameters.RetryAfter) * time.Second}
	}
	if res.StatusCode >= 300 || !reply.OK {
		return fmt.Errorf("telegram API %d: %s", res.StatusCode, reply.Description)
	}
	return nil
}

// CheckTelegram calls getMe once at startup, like upstream's healthcheck.
func (n *Notifier) CheckTelegram(ctx context.Context) {
	if !n.cfg.TelegramEnabled {
		return
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, n.cfg.TelegramAPIRoot+"/bot"+n.cfg.TelegramBotToken+"/getMe", nil)
	if err != nil {
		return
	}
	res, err := n.telegram.Do(req)
	if err != nil {
		log.Printf("[NOTIFY] Telegram getMe failed: network error")
		return
	}
	defer res.Body.Close()
	var reply struct {
		OK     bool `json:"ok"`
		Result struct {
			Username string `json:"username"`
		} `json:"result"`
	}
	_ = json.NewDecoder(res.Body).Decode(&reply)
	if !reply.OK {
		log.Printf("[NOTIFY] Telegram getMe failed with status %d", res.StatusCode)
		return
	}
	log.Printf("[NOTIFY] Telegram notifications enabled. Bot username: %s", reply.Result.Username)
}

var current atomic.Pointer[Notifier]

// SetDefault installs the process-wide notifier used by Emit.
func SetDefault(n *Notifier) { current.Store(n) }

// Emit enqueues events through the default notifier; without one (tests,
// CLI commands) it does nothing.
func Emit(tx *gorm.DB, events ...Event) error {
	if n := current.Load(); n != nil {
		return n.Emit(tx, events...)
	}
	return nil
}
