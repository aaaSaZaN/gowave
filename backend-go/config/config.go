package config

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	AppPort         string
	MetricsPort     string
	AppSecret       string
	DBDriver        string
	DatabaseURL     string
	FrontEndDomain  string
	PanelDomain     string
	SubPublicDomain string
	JWTLifetime     int

	Notifications NotificationsConfig
	Service       ServiceConfig
	// ShortUUIDLength is SHORT_UUID_LENGTH (16-64).
	ShortUUIDLength int
	// UserUsageIgnoreBelowBytes is USER_USAGE_IGNORE_BELOW_BYTES.
	UserUsageIgnoreBelowBytes int64
	MetricsUser               string
	MetricsPass               string

	errors []string
}

// NotificationsConfig mirrors the upstream webhook, Telegram and threshold
// variables. A nil slice means the feature is disabled.
type NotificationsConfig struct {
	WebhookEnabled      bool
	WebhookURLs         []string
	WebhookSecretHeader string

	TelegramEnabled  bool
	TelegramBotToken string
	TelegramAPIRoot  string
	TelegramProxy    string
	// Telegram chat targets in upstream "chatId" or "chatId:threadId" form.
	TelegramUsers    string
	TelegramNodes    string
	TelegramCRM      string
	TelegramService  string
	TelegramTBlocker string

	BandwidthUsageThresholds []int
	NotConnectedAfterHours   []int
	ExpirationNotifications  []int
}

type ServiceConfig struct {
	CleanUsageHistory       bool
	DisableUserUsageRecords bool
	DisableSrhRecords       bool
	ExportToStream          bool
}

func Load() *Config {
	_ = godotenv.Load(".env", "../.env")

	port := os.Getenv("APP_PORT")
	if port == "" {
		port = "3000"
	}

	metricsPort := os.Getenv("METRICS_PORT")
	if metricsPort == "" {
		metricsPort = "3001"
	}

	secret := os.Getenv("APP_SECRET")

	dbDriver := strings.ToLower(os.Getenv("DB_DRIVER"))
	dbURL := os.Getenv("DATABASE_URL")

	if dbDriver == "" {
		if strings.HasPrefix(dbURL, "postgres://") || strings.HasPrefix(dbURL, "postgresql://") {
			dbDriver = "postgres"
		} else {
			dbDriver = "sqlite"
		}
	}

	if dbURL == "" {
		if dbDriver == "postgres" {
			dbURL = "postgresql://postgres:postgres@localhost:5432/remnawave"
		} else {
			dbURL = "remnawave.db"
		}
	}

	feDomain := os.Getenv("FRONT_END_DOMAIN")
	if feDomain == "" {
		feDomain = "*"
	}

	panelDomain := os.Getenv("PANEL_DOMAIN")
	if panelDomain == "" {
		panelDomain = "localhost:3000"
	}

	subDomain := os.Getenv("SUB_PUBLIC_DOMAIN")
	if subDomain == "" {
		subDomain = "localhost:3000/api/sub"
	}

	jwtLifetime := 12
	if lifetimeStr := os.Getenv("JWT_AUTH_LIFETIME"); lifetimeStr != "" {
		if val, err := strconv.Atoi(lifetimeStr); err == nil && val > 0 {
			jwtLifetime = val
		}
	}

	cfg := &Config{
		AppPort:         port,
		MetricsPort:     metricsPort,
		AppSecret:       secret,
		DBDriver:        dbDriver,
		DatabaseURL:     dbURL,
		FrontEndDomain:  feDomain,
		PanelDomain:     panelDomain,
		SubPublicDomain: subDomain,
		JWTLifetime:     jwtLifetime,
		MetricsUser:     os.Getenv("METRICS_USER"),
		MetricsPass:     os.Getenv("METRICS_PASS"),
	}
	cfg.loadOptional()
	return cfg
}

func envBool(key string) bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv(key)), "true")
}

// envList parses a JSON number array such as "[60, 80]"; it returns nil when
// the gate variable is false, matching upstream getIfEnabled.
func (c *Config) envList(gate, key string) []int {
	if !envBool(gate) {
		return nil
	}
	raw := strings.TrimSpace(os.Getenv(key))
	var values []int
	if raw == "" || json.Unmarshal([]byte(raw), &values) != nil || len(values) == 0 {
		c.errors = append(c.errors, key+" must be a non-empty JSON array when "+gate+" is true")
		return nil
	}
	return values
}

func (c *Config) loadOptional() {
	n := &c.Notifications
	n.WebhookEnabled = envBool("WEBHOOK_ENABLED")
	n.WebhookSecretHeader = os.Getenv("WEBHOOK_SECRET_HEADER")
	if n.WebhookEnabled {
		for _, url := range strings.Split(os.Getenv("WEBHOOK_URL"), ",") {
			url = strings.TrimSpace(url)
			if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
				c.errors = append(c.errors, "WEBHOOK_URL must start with http:// or https://")
				continue
			}
			n.WebhookURLs = append(n.WebhookURLs, url)
		}
		if len(n.WebhookSecretHeader) < 32 || !regexp.MustCompile(`^[a-zA-Z0-9]+$`).MatchString(n.WebhookSecretHeader) {
			c.errors = append(c.errors, "WEBHOOK_SECRET_HEADER must be at least 32 letters and numbers when WEBHOOK_ENABLED is true")
		}
	}
	n.TelegramEnabled = envBool("IS_TELEGRAM_NOTIFICATIONS_ENABLED")
	n.TelegramBotToken = os.Getenv("TELEGRAM_BOT_TOKEN")
	n.TelegramAPIRoot = strings.TrimRight(os.Getenv("TELEGRAM_BOT_API_ROOT"), "/")
	if n.TelegramAPIRoot == "" {
		n.TelegramAPIRoot = "https://api.telegram.org"
	}
	n.TelegramProxy = os.Getenv("TELEGRAM_BOT_PROXY")
	n.TelegramUsers = os.Getenv("TELEGRAM_NOTIFY_USERS")
	n.TelegramNodes = os.Getenv("TELEGRAM_NOTIFY_NODES")
	n.TelegramCRM = os.Getenv("TELEGRAM_NOTIFY_CRM")
	n.TelegramService = os.Getenv("TELEGRAM_NOTIFY_SERVICE")
	n.TelegramTBlocker = os.Getenv("TELEGRAM_NOTIFY_TBLOCKER")
	if n.TelegramEnabled && (n.TelegramBotToken == "" || n.TelegramBotToken == "change_me") {
		c.errors = append(c.errors, "TELEGRAM_BOT_TOKEN is required when IS_TELEGRAM_NOTIFICATIONS_ENABLED is true")
	}
	if n.TelegramProxy == "change_me" {
		c.errors = append(c.errors, `TELEGRAM_BOT_PROXY cannot be set to "change_me"`)
	}
	n.BandwidthUsageThresholds = c.envList("BANDWIDTH_USAGE_NOTIFICATIONS_ENABLED", "BANDWIDTH_USAGE_NOTIFICATIONS_THRESHOLD")
	n.NotConnectedAfterHours = c.envList("NOT_CONNECTED_USERS_NOTIFICATIONS_ENABLED", "NOT_CONNECTED_USERS_NOTIFICATIONS_AFTER_HOURS")
	n.ExpirationNotifications = c.envList("EXPIRATION_NOTIFICATIONS_ENABLED", "EXPIRATION_NOTIFICATIONS")

	c.Service = ServiceConfig{
		CleanUsageHistory:       envBool("SERVICE_CLEAN_USAGE_HISTORY"),
		DisableUserUsageRecords: envBool("SERVICE_DISABLE_USER_USAGE_RECORDS"),
		DisableSrhRecords:       envBool("SERVICE_DISABLE_SRH_RECORDS"),
		ExportToStream:          envBool("EXPORT_TO_STREAM_ENABLED"),
	}

	c.ShortUUIDLength = 16
	if raw := os.Getenv("SHORT_UUID_LENGTH"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 16 || value > 64 {
			c.errors = append(c.errors, "SHORT_UUID_LENGTH must be between 16 and 64")
		} else {
			c.ShortUUIDLength = value
		}
	}
	if raw := os.Getenv("USER_USAGE_IGNORE_BELOW_BYTES"); raw != "" {
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || value < 0 {
			c.errors = append(c.errors, "USER_USAGE_IGNORE_BELOW_BYTES must be a non-negative integer")
		} else {
			c.UserUsageIgnoreBelowBytes = value
		}
	}
}

// Validate prevents accidentally deploying with a public or weak signing key.
func (c *Config) Validate() error {
	if len(c.AppSecret) < 32 || c.AppSecret == "change_me" || c.AppSecret == "remnawave-default-secret-key-32chars-minimum!!" {
		return fmt.Errorf("APP_SECRET must be a private, randomly generated value of at least 32 characters")
	}
	if c.DBDriver != "sqlite" && c.DBDriver != "postgres" {
		return fmt.Errorf("DB_DRIVER must be sqlite or postgres")
	}
	if len(c.errors) > 0 {
		return fmt.Errorf("%s", strings.Join(c.errors, "; "))
	}
	return nil
}
