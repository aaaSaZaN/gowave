package config

import "testing"

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
