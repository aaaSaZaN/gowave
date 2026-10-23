package server

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
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
			if serveDashboardFile(w, r, local) {
				return
			}
			// Editor runtimes and schemas historically also live in the static root.
			if strings.HasPrefix(relative, "assets/") &&
				serveDashboardFile(w, r, filepath.Join(dir, filepath.FromSlash(strings.TrimPrefix(relative, "assets/")))) {
				return
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

// serveDashboardFile serves a regular file, preferring a precompressed name.gz
// sibling produced by the frontend build. The Xray validator runtime is shipped
// only in compressed form, so clients that refuse gzip get it decoded here.
func serveDashboardFile(w http.ResponseWriter, r *http.Request, name string) bool {
	plain, err := os.Stat(name)
	hasPlain := err == nil && !plain.IsDir()
	packed, err := os.Stat(name + ".gz")
	hasPacked := err == nil && !packed.IsDir()
	if !hasPlain && !hasPacked {
		return false
	}
	if hasPacked {
		w.Header().Add("Vary", "Accept-Encoding")
	}
	if !hasPacked || (hasPlain && !acceptsGzip(r)) {
		f, err := os.Open(name)
		if err != nil {
			http.NotFound(w, r)
			return true
		}
		defer f.Close()
		http.ServeContent(w, r, name, plain.ModTime(), f)
		return true
	}

	f, err := os.Open(name + ".gz")
	if err != nil {
		http.NotFound(w, r)
		return true
	}
	defer f.Close()
	if ctype := mime.TypeByExtension(path.Ext(name)); ctype != "" {
		w.Header().Set("Content-Type", ctype)
	}
	if acceptsGzip(r) {
		w.Header().Set("Content-Encoding", "gzip")
		http.ServeContent(w, r, name, packed.ModTime(), f)
		return true
	}
	zr, err := gzip.NewReader(f)
	if err != nil {
		http.Error(w, "corrupt asset", http.StatusInternalServerError)
		return true
	}
	defer zr.Close()
	w.Header().Set("Last-Modified", packed.ModTime().UTC().Format(http.TimeFormat))
	if r.Method != http.MethodHead {
		_, _ = io.Copy(w, zr)
	}
	return true
}

func acceptsGzip(r *http.Request) bool {
	for _, part := range strings.Split(r.Header.Get("Accept-Encoding"), ",") {
		coding, params, _ := strings.Cut(part, ";")
		if !strings.EqualFold(strings.TrimSpace(coding), "gzip") {
			continue
		}
		key, value, ok := strings.Cut(strings.TrimSpace(params), "=")
		if !ok || strings.TrimSpace(key) != "q" {
			return true
		}
		q, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		return err != nil || q > 0
	}
	return false
}
