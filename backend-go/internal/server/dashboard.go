package server

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"gowave/config"
)

// dashboardHandler confines the SPA and its assets to the configured prefix.
// API routes are registered separately and retain their existing paths.
func dashboardHandler(dir, configuredBase string) http.Handler {
	base, err := config.NormalizePanelBasePath(configuredBase)
	if err != nil {
		return http.NotFoundHandler()
	}
	files := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if base != "/" && r.URL.Path == strings.TrimSuffix(base, "/") {
			target := base
			if r.URL.RawQuery != "" {
				target += "?" + r.URL.RawQuery
			}
			http.Redirect(w, r, target, http.StatusPermanentRedirect)
			return
		}
		if !strings.HasPrefix(r.URL.Path, base) {
			http.NotFound(w, r)
			return
		}
		relative := strings.TrimPrefix(r.URL.Path, base)
		if relative != "" && !fs.ValidPath(strings.TrimSuffix(relative, "/")) {
			http.NotFound(w, r)
			return
		}
		local := filepath.Join(dir, filepath.FromSlash(relative))
		if relative == "site.webmanifest" {
			data, err := os.ReadFile(local)
			var manifest map[string]interface{}
			if err != nil || json.Unmarshal(data, &manifest) != nil {
				http.NotFound(w, r)
				return
			}
			manifest["start_url"], manifest["scope"], manifest["id"] = base, base, base
			w.Header().Set("Content-Type", "application/manifest+json")
			w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
			if r.Method != http.MethodHead {
				_ = json.NewEncoder(w).Encode(manifest)
			}
			return
		}
		if relative != "" && relative != "index.html" {
			if info, err := os.Stat(local); err == nil && !info.IsDir() {
				cloned := r.Clone(r.Context())
				cloned.URL.Path = "/" + relative
				cloned.URL.RawPath = ""
				files.ServeHTTP(w, cloned)
				return
			}
			// Editor runtimes and schemas historically also live in the static root.
			if strings.HasPrefix(relative, "assets/") {
				alt := filepath.Join(dir, filepath.FromSlash(strings.TrimPrefix(relative, "assets/")))
				if info, err := os.Stat(alt); err == nil && !info.IsDir() {
					http.ServeFile(w, r, alt)
					return
				}
			}
			if ext := path.Ext(relative); ext != "" && ext != ".html" {
				http.NotFound(w, r)
				return
			}
		}
		data, err := os.ReadFile(filepath.Join(dir, "index.html"))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		// The marker is emitted by Vite and contains no build-specific URL.
		data = bytes.Replace(data, []byte(`href="/" data-gowave-base`), []byte(`href="`+base+`" data-gowave-base`), 1)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		if r.Method != http.MethodHead {
			_, _ = w.Write(data)
		}
	})
}
