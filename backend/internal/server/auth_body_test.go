package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPublicAuthBodyLimit(t *testing.T) {
	app := newTestApp(t)
	server := httptest.NewServer(app.Router)
	t.Cleanup(server.Close)
	const limit = 16 << 10
	for _, route := range []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/auth/status"},
		{http.MethodHead, "/api/auth/status"},
		{http.MethodPost, "/api/auth/login"},
		{http.MethodPost, "/api/auth/register"},
		{http.MethodGet, "/api/auth/passkey/authentication/options"},
		{http.MethodPost, "/api/auth/passkey/authentication/verify"},
		{http.MethodPost, "/api/auth/oauth2/authorize"},
		{http.MethodPost, "/api/auth/oauth2/callback"},
		{http.MethodPost, "/api/auth/oauth2/tg/callback"},
	} {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			baseline := httptest.NewRecorder()
			app.Router.ServeHTTP(baseline, httptest.NewRequest(route.method, route.path, strings.NewReader(`{}`)))
			for _, size := range []int{limit - 1, limit, limit + 1, 1 << 20} {
				for _, chunked := range []bool{false, true} {
					t.Run(fmt.Sprintf("size=%d/chunked=%t", size, chunked), func(t *testing.T) {
						// A decodable prefix must not hide an oversized suffix.
						payload := `{}` + strings.Repeat(" ", size-2)
						req, err := http.NewRequest(route.method, server.URL+route.path, strings.NewReader(payload))
						if err != nil {
							t.Fatal(err)
						}
						if chunked {
							req.ContentLength = -1
							req.TransferEncoding = []string{"chunked"}
						}
						req.Header.Set("Content-Type", "application/json")
						res, err := server.Client().Do(req)
						if err != nil {
							t.Fatal(err)
						}
						defer res.Body.Close()
						body, err := io.ReadAll(res.Body)
						if err != nil {
							t.Fatal(err)
						}
						want := baseline.Code
						if size > limit {
							want = http.StatusRequestEntityTooLarge
							if route.method != http.MethodHead {
								var envelope struct {
									Timestamp string `json:"timestamp"`
									Path      string `json:"path"`
									Message   string `json:"message"`
									ErrorCode string `json:"errorCode"`
								}
								if err := json.Unmarshal(body, &envelope); err != nil {
									t.Fatal(err)
								}
								if envelope.Path != route.path || envelope.Timestamp == "" || envelope.Message != "Request body too large" || envelope.ErrorCode != "E000" {
									t.Fatalf("unexpected error envelope: %s", body)
								}
							}
						}
						if res.StatusCode != want {
							t.Fatalf("status=%d, want %d: %s", res.StatusCode, want, body)
						}
					})
				}
			}
		})
	}
}
