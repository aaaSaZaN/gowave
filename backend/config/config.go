package config

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	// InstallDir is the root directory of the Gowave installation.
	InstallDir string
	// KeepRootDB preserves root ownership of database files (skips chown to gowave).
	KeepRootDB bool
	// AppHost is the address the panel and metrics listeners bind to. It
	// defaults to loopback so the panel is only reachable through a reverse
	// proxy with HTTPS; Docker sets 0.0.0.0 inside the container.
	AppHost         string
	AppPort         string
	MetricsPort     string
	MetricsEnabled  bool // METRICS_ENABLED=false keeps METRICS_PORT closed
	AppSecret       string
	DBDriver        string
	DatabaseURL     string
	FrontEndDomain  string
	PanelDomain     string
	PanelBasePath   string
	SubPublicDomain string
	JWTLifetime     int
	// HTTPLogging is IS_HTTP_LOGGING_ENABLED; like upstream it is off by
	// default because every request, subscription fetches included, would
	// otherwise reach the service log.
	HTTPLogging bool
	// LogLevel is LOG_LEVEL (debug, info, warn, error) and LogFormat is
	// LOG_FORMAT (text, json) for the service log.
	LogLevel  string
	LogFormat string

	Notifications NotificationsConfig
	Service       ServiceConfig
	// ShortUUIDLength is SHORT_UUID_LENGTH (16-64).
	ShortUUIDLength int
	// UserUsageIgnoreBelowBytes is USER_USAGE_IGNORE_BELOW_BYTES.
	UserUsageIgnoreBelowBytes int64
	// DBMaxConnections is DB_MAX_CONNECTIONS, the PostgreSQL pool size; 0
	// keeps database.DefaultMaxConnections.
	DBMaxConnections int
	MetricsUser      string
	MetricsPass      string

	// SubUserCacheSize is SUB_USER_CACHE_SIZE: max users cached in RAM (default 10000, 0 to disable).
	SubUserCacheSize int
	// SubUserCacheTTL is SUB_USER_CACHE_TTL: TTL for cached user data (supports s, m, h, d; default 5s).
	SubUserCacheTTL time.Duration

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

// hasGowaveMarkers checks if a directory contains characteristic Gowave files.
func hasGowaveMarkers(dir string) bool {
	markers := []string{".env", "gowave.db", "gowave-server", "gowave", "go.mod"}
	for _, m := range markers {
		if _, err := os.Stat(filepath.Join(dir, m)); err == nil {
			return true
		}
	}
	return false
}

// DetectInstallDir locates the Gowave installation root directory.
// It inspects GOWAVE_INSTALL_DIR, the resolved executable path (following symlinks
// such as /usr/local/bin/gowave -> /opt/gowave/gowave-server), the current working
// directory and parent directory, or standard install locations (/opt/gowave).
func DetectInstallDir() string {
	if envDir := strings.TrimSpace(os.Getenv("GOWAVE_INSTALL_DIR")); envDir != "" {
		if fi, err := os.Stat(envDir); err == nil && fi.IsDir() {
			if abs, err := filepath.Abs(envDir); err == nil {
				return abs
			}
			return envDir
		}
	}

	if exe, err := os.Executable(); err == nil {
		if resolved, err := filepath.EvalSymlinks(exe); err == nil {
			exe = resolved
		}
		dir := filepath.Dir(exe)
		tmp := os.TempDir()
		if !strings.Contains(dir, "go-build") && dir != tmp && dir != "/usr/bin" && dir != "/usr/local/bin" && dir != "/bin" {
			if hasGowaveMarkers(dir) || (!strings.HasPrefix(dir, tmp) && dir != "/" && dir != "/tmp") {
				if abs, err := filepath.Abs(dir); err == nil {
					return abs
				}
				return dir
			}
		}
	}

	if cwd, err := os.Getwd(); err == nil {
		if hasGowaveMarkers(cwd) {
			return cwd
		}
		parent := filepath.Dir(cwd)
		if hasGowaveMarkers(parent) {
			return parent
		}
	}

	defaultDir := "/opt/gowave"
	if fi, err := os.Stat(defaultDir); err == nil && fi.IsDir() {
		return defaultDir
	}

	return ""
}

func Load() *Config {
	installDir := DetectInstallDir()

	var envFiles []string
	if _, err := os.Stat(".env"); err == nil {
		envFiles = append(envFiles, ".env")
	}
	if _, err := os.Stat("../.env"); err == nil {
		envFiles = append(envFiles, "../.env")
	}
	if len(envFiles) == 0 && installDir != "" {
		installEnv := filepath.Join(installDir, ".env")
		if _, err := os.Stat(installEnv); err == nil {
			envFiles = append(envFiles, installEnv)
		}
	}
	if len(envFiles) == 0 {
		if _, err := os.Stat("/opt/gowave/.env"); err == nil {
			envFiles = append(envFiles, "/opt/gowave/.env")
		}
	}

	if len(envFiles) > 0 {
		_ = godotenv.Load(envFiles...)
	} else {
		_ = godotenv.Load(".env", "../.env")
	}

	host := strings.TrimSpace(os.Getenv("APP_HOST"))
	if host == "" {
		host = "127.0.0.1"
	}

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
			dbURL = "postgresql://postgres:postgres@localhost:5432/gowave"
		} else {
			dbURL = "gowave.db"
		}
	}

	if dbDriver != "postgres" && !filepath.IsAbs(dbURL) && !strings.HasPrefix(dbURL, "file:") && dbURL != ":memory:" {
		if installDir != "" {
			candidate := filepath.Join(installDir, dbURL)
			if _, err := os.Stat(candidate); err == nil {
				dbURL = candidate
			} else if _, err := os.Stat(dbURL); os.IsNotExist(err) {
				dbURL = candidate
			}
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
		InstallDir:      installDir,
		AppHost:         host,
		AppPort:         port,
		MetricsPort:     metricsPort,
		MetricsEnabled:  !strings.EqualFold(strings.TrimSpace(os.Getenv("METRICS_ENABLED")), "false"),
		AppSecret:       secret,
		DBDriver:        dbDriver,
		DatabaseURL:     dbURL,
		FrontEndDomain:  feDomain,
		PanelDomain:     panelDomain,
		PanelBasePath:   os.Getenv("PANEL_BASE_PATH"),
		SubPublicDomain: subDomain,
		JWTLifetime:     jwtLifetime,
		HTTPLogging:     envBool("IS_HTTP_LOGGING_ENABLED"),
		LogLevel:        os.Getenv("LOG_LEVEL"),
		LogFormat:       os.Getenv("LOG_FORMAT"),
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
	// Upstream's .env sample ships "change_me" for every chat; an unedited one
	// means the scope is not configured, not a chat to retry sending to.
	telegramTarget := func(key string) string {
		value := strings.TrimSpace(os.Getenv(key))
		if value == "change_me" {
			return ""
		}
		return value
	}
	n.TelegramUsers = telegramTarget("TELEGRAM_NOTIFY_USERS")
	n.TelegramNodes = telegramTarget("TELEGRAM_NOTIFY_NODES")
	n.TelegramCRM = telegramTarget("TELEGRAM_NOTIFY_CRM")
	n.TelegramService = telegramTarget("TELEGRAM_NOTIFY_SERVICE")
	n.TelegramTBlocker = telegramTarget("TELEGRAM_NOTIFY_TBLOCKER")
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
	if raw := os.Getenv("DB_MAX_CONNECTIONS"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > 500 {
			c.errors = append(c.errors, "DB_MAX_CONNECTIONS must be between 1 and 500")
		} else {
			c.DBMaxConnections = value
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

	c.SubUserCacheSize = 10000
	if raw := os.Getenv("SUB_USER_CACHE_SIZE"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 0 {
			c.errors = append(c.errors, "SUB_USER_CACHE_SIZE must be a non-negative integer")
		} else {
			c.SubUserCacheSize = value
		}
	}

	c.SubUserCacheTTL = 5 * time.Second
	if raw := os.Getenv("SUB_USER_CACHE_TTL"); raw != "" {
		if d, err := parseDurationWithDays(raw); err == nil && d > 0 {
			c.SubUserCacheTTL = d
		} else {
			c.errors = append(c.errors, `SUB_USER_CACHE_TTL must be a valid duration (e.g. "2d", "12h", "30m", "15s")`)
		}
	}
}

var dayDurationPattern = regexp.MustCompile(`(?i)(\d+(?:\.\d+)?)d`)

func parseDurationWithDays(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	expanded := dayDurationPattern.ReplaceAllStringFunc(s, func(match string) string {
		numStr := match[:len(match)-1]
		val, err := strconv.ParseFloat(numStr, 64)
		if err != nil {
			return match
		}
		hours := val * 24
		return fmt.Sprintf("%fh", hours)
	})
	return time.ParseDuration(expanded)
}

// IsLoopbackHost reports whether AppHost only accepts local connections.
func (c *Config) IsLoopbackHost() bool {
	host := strings.Trim(c.AppHost, "[]")
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// Validate prevents accidentally deploying with a public or weak signing key.
// NormalizePanelBasePath validates a dashboard-only mount point.
func NormalizePanelBasePath(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || value == "/" {
		return "/", nil
	}
	// "panel", "/panel" and "/panel/" all mean the same mount point.
	if !strings.HasPrefix(value, "/") {
		value = "/" + value
	}
	if !regexp.MustCompile(`^/[a-zA-Z0-9_-]+(/[a-zA-Z0-9_-]+)*/?$`).MatchString(value) {
		return "", fmt.Errorf("PANEL_BASE_PATH must use letters, numbers, underscores, hyphens and single slashes, for example /panel/")
	}
	first := strings.Split(strings.Trim(value, "/"), "/")[0]
	if first == "api" || first == "metrics" {
		return "", fmt.Errorf("PANEL_BASE_PATH overlaps a backend endpoint")
	}
	return strings.TrimRight(value, "/") + "/", nil
}

func (c *Config) Validate() error {
	base, err := NormalizePanelBasePath(c.PanelBasePath)
	if err != nil {
		return err
	}
	c.PanelBasePath = base
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
