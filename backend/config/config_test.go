package config

import (
	"strings"
	"testing"
	"time"
)

func TestAppHostDefaultsToLoopback(t *testing.T) {
	t.Setenv("APP_HOST", "")
	if cfg := Load(); cfg.AppHost != "127.0.0.1" || !cfg.IsLoopbackHost() {
		t.Fatalf("AppHost = %q, want 127.0.0.1", cfg.AppHost)
	}
}

func TestIsLoopbackHost(t *testing.T) {
	for host, want := range map[string]bool{
		"127.0.0.1": true,
		"::1":       true,
		"[::1]":     true,
		"localhost": true,
		"0.0.0.0":   false,
		"":          false,
		"10.0.0.5":  false,
	} {
		if got := (&Config{AppHost: host}).IsLoopbackHost(); got != want {
			t.Errorf("IsLoopbackHost(%q) = %v, want %v", host, got, want)
		}
	}
}

func TestDBMaxConnections(t *testing.T) {
	t.Setenv("APP_SECRET", "0123456789abcdef0123456789abcdef")
	t.Setenv("DB_MAX_CONNECTIONS", "20")
	if cfg := Load(); cfg.DBMaxConnections != 20 || cfg.Validate() != nil {
		t.Fatalf("DB_MAX_CONNECTIONS=20: got %d, %v", cfg.DBMaxConnections, cfg.Validate())
	}
	for _, bad := range []string{"0", "-1", "501", "many"} {
		t.Setenv("DB_MAX_CONNECTIONS", bad)
		if err := Load().Validate(); err == nil || !strings.Contains(err.Error(), "DB_MAX_CONNECTIONS") {
			t.Errorf("DB_MAX_CONNECTIONS=%q should be rejected, got %v", bad, err)
		}
	}
}

func TestMetricsEnabled(t *testing.T) {
	for value, want := range map[string]bool{"": true, "true": true, "false": false, " FALSE ": false} {
		t.Setenv("METRICS_ENABLED", value)
		if got := Load().MetricsEnabled; got != want {
			t.Fatalf("METRICS_ENABLED=%q: got %v, want %v", value, got, want)
		}
	}
}

func TestSubUserCacheConfig(t *testing.T) {
	t.Setenv("APP_SECRET", "0123456789abcdef0123456789abcdef")

	// Defaults: size 10000, ttl 2d (48h)
	t.Setenv("SUB_USER_CACHE_SIZE", "")
	t.Setenv("SUB_USER_CACHE_TTL", "")
	cfg := Load()
	if cfg.SubUserCacheSize != 10000 {
		t.Fatalf("SubUserCacheSize default = %d, want 10000", cfg.SubUserCacheSize)
	}
	if cfg.SubUserCacheTTL != 48*time.Hour {
		t.Fatalf("SubUserCacheTTL default = %v, want 48h", cfg.SubUserCacheTTL)
	}

	// Supports s, m, h, d
	cases := map[string]time.Duration{
		"2d":    48 * time.Hour,
		"1d":    24 * time.Hour,
		"3d":    72 * time.Hour,
		"0.5d":  12 * time.Hour,
		"12h":   12 * time.Hour,
		"30m":   30 * time.Minute,
		"15s":   15 * time.Second,
		"1d12h": 36 * time.Hour,
	}
	for raw, want := range cases {
		t.Setenv("SUB_USER_CACHE_TTL", raw)
		c := Load()
		if c.SubUserCacheTTL != want || c.Validate() != nil {
			t.Errorf("SUB_USER_CACHE_TTL=%q got %v (err: %v), want %v", raw, c.SubUserCacheTTL, c.Validate(), want)
		}
	}

	// Invalid durations
	for _, bad := range []string{"-1s", "0s", "invalid", "2weeks"} {
		t.Setenv("SUB_USER_CACHE_TTL", bad)
		if err := Load().Validate(); err == nil || !strings.Contains(err.Error(), "SUB_USER_CACHE_TTL") {
			t.Errorf("SUB_USER_CACHE_TTL=%q should be rejected, got %v", bad, err)
		}
	}
}
