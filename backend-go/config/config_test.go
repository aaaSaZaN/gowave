package config

import (
	"strings"
	"testing"
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
