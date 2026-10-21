package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gowave/config"
	"gowave/internal/testdb"
)

func dashboardFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range map[string]string{
		"index.html":    `<html><head><base href="/" data-gowave-base /><script type="module" src="./assets/app.js"></script></head></html>`,
		"assets/app.js": "window.loaded=true", "main.wasm": "wasm", "locales/en/gowave.json": `{"hello":"GoWave"}`,
		"site.webmanifest": `{"start_url":"./","scope":"./","icons":[{"src":"./favicons/logo.svg"}]}`,
	} {
		file := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(file), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestDashboardMount(t *testing.T) {
	dir := dashboardFixture(t)
	for _, base := range []string{"/", "/private/panel/"} {
		t.Run(base, func(t *testing.T) {
			h := dashboardHandler(dir, base)
			for _, suffix := range []string{"", "index.html", "dashboard/home", "dashboard/home/", "auth/login", "oauth2/callback/github"} {
				w := httptest.NewRecorder()
				h.ServeHTTP(w, httptest.NewRequest("GET", base+suffix, nil))
				if w.Code != 200 || !strings.Contains(w.Body.String(), `href="`+base+`" data-gowave-base`) {
					t.Fatalf("%s: %d %s", suffix, w.Code, w.Body.String())
				}
				if !strings.Contains(w.Header().Get("Cache-Control"), "no-store") {
					t.Fatal("HTML is cacheable")
				}
			}
			for suffix, want := range map[string]string{"assets/app.js": "window.loaded=true", "assets/main.wasm": "wasm", "locales/en/gowave.json": `{"hello":"GoWave"}`} {
				w := httptest.NewRecorder()
				h.ServeHTTP(w, httptest.NewRequest("GET", base+suffix, nil))
				if w.Code != 200 || w.Body.String() != want {
					t.Fatalf("asset %s: %d %s", suffix, w.Code, w.Body.String())
				}
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, httptest.NewRequest("GET", base+"assets/missing.js", nil))
			if w.Code != 404 {
				t.Fatal("missing asset returned SPA")
			}
			w = httptest.NewRecorder()
			h.ServeHTTP(w, httptest.NewRequest("GET", base+"site.webmanifest", nil))
			if w.Code != 200 || !strings.Contains(w.Body.String(), `"scope":"`+base+`"`) {
				t.Fatal(w.Body.String())
			}
			for _, suffix := range []string{"dashboard/home", "site.webmanifest"} {
				w = httptest.NewRecorder()
				h.ServeHTTP(w, httptest.NewRequest("HEAD", base+suffix, nil))
				if w.Code != 200 || w.Body.Len() != 0 {
					t.Fatal("HEAD returned body or failed")
				}
			}
			if base != "/" {
				for _, outside := range []string{"/", "/dashboard/home", "/assets/app.js", "/private/panel-other", "/private/panel/../index.html"} {
					w = httptest.NewRecorder()
					h.ServeHTTP(w, httptest.NewRequest("GET", outside, nil))
					if w.Code != 404 {
						t.Errorf("outside path %s: %d", outside, w.Code)
					}
				}
				w = httptest.NewRecorder()
				h.ServeHTTP(w, httptest.NewRequest("GET", strings.TrimSuffix(base, "/")+"?a=1", nil))
				if w.Code != http.StatusPermanentRedirect || w.Header().Get("Location") != base+"?a=1" {
					t.Fatal("prefix redirect", w.Code, w.Header())
				}
			}
		})
	}
}

func TestDashboardPrefixPreservesAPI(t *testing.T) {
	// NewApp resolves ./dist at construction time; all tests in this package are sequential.
	t.Chdir(t.TempDir())
	dir := dashboardFixture(t)
	if err := os.Rename(dir, "dist"); err != nil {
		t.Fatal(err)
	}
	app := NewApp(&config.Config{AppSecret: strings.Repeat("s", 32), DBDriver: "sqlite", PanelBasePath: "/private/panel/"}, testdb.Open(t))
	for url, status := range map[string]int{"/": 404, "/private/panel/": 200, "/api/system/health": 200, "/api/users": 401} {
		w := httptest.NewRecorder()
		app.Router.ServeHTTP(w, httptest.NewRequest("GET", url, nil))

		if w.Code != status {
			t.Errorf("%s: got %d want %d", url, w.Code, status)
		}
	}
}
