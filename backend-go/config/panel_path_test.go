package config

import "testing"

func TestPanelBasePath(t *testing.T) {
	for input, want := range map[string]string{"": "/", "/": "/", "/panel": "/panel/", "/private/panel/": "/private/panel/", "/a_B-123": "/a_B-123/"} {
		got, err := NormalizePanelBasePath(input)
		if err != nil || got != want {
			t.Errorf("%q: got %q, %v; want %q", input, got, err, want)
		}
	}
	for _, input := range []string{"panel", "//evil", "/../panel", "/a/./b", "/a//b", "/api", "/api/sub/", "/metrics/", "/p?x=y", "/p#x", "/a%2fb", "/a.b", "/<script>", "/space here"} {
		if _, err := NormalizePanelBasePath(input); err == nil {
			t.Errorf("accepted invalid path %q", input)
		}
	}
}

func TestLoadPanelBasePath(t *testing.T) {
	t.Setenv("PANEL_BASE_PATH", "/private/panel")
	cfg := Load()
	cfg.AppSecret = "0123456789abcdef0123456789abcdef"
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	if cfg.PanelBasePath != "/private/panel/" {
		t.Fatal(cfg.PanelBasePath)
	}
}
