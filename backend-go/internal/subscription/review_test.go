package subscription

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"remnawave-go/internal/middleware"
)

func TestRawProtectedAnnounceOnlyForDeviceLimit(t *testing.T) {
	db, user := subscriptionDB(t)
	setSettings(t, db, map[string]interface{}{"hwid_settings": `{"enabled":true,"fallbackDeviceLimit":1,"maxDevicesAnnounce":"Too many for {{USERNAME}}"}`})
	handler := NewHandler(db, "sub.example")
	router := chi.NewRouter()
	router.Get("/raw/{shortUuid}", handler.RawProtected)
	for _, tc := range []struct{ hwid, announce string }{
		{"", ""},
		{"device-one-123", ""},
		{"device-two-123", "base64:" + base64.StdEncoding.EncodeToString([]byte("Too many for subscriber"))},
	} {
		headers := map[string]string{"User-Agent": "test-client"}
		if tc.hwid != "" {
			headers["X-Hwid"] = tc.hwid
		}
		w := get(router, "/raw/"+user.ShortUUID, headers)
		var output struct {
			Response struct {
				Headers map[string]string `json:"headers"`
			} `json:"response"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &output); err != nil {
			t.Fatal(err)
		}
		if got := output.Response.Headers["announce"]; got != tc.announce {
			t.Errorf("hwid %q announce = %q, want %q; body %s", tc.hwid, got, tc.announce, w.Body.String())
		}
	}
}

func TestSocketDropThroughContractStatus(t *testing.T) {
	db, user := subscriptionDB(t)
	setSettings(t, db, map[string]interface{}{"response_rules": `{"version":"1","rules":[{"name":"drop","enabled":true,"operator":"AND","conditions":[],"responseType":"SOCKET_DROP"}]}`})
	handler := middleware.ContractStatus(subscriptionRouter(db))
	// A writer without hijacking must abort instead of producing a 200.
	func() {
		defer func() {
			if value := recover(); value != http.ErrAbortHandler {
				t.Fatalf("hijack failure panic = %v", value)
			}
		}()
		get(handler, "/api/sub/"+user.ShortUUID, curl)
	}()
	server := httptest.NewServer(handler)
	defer server.Close()
	req, err := http.NewRequest("GET", server.URL+"/api/sub/"+user.ShortUUID, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("User-Agent", "curl/8.0")
	response, err := server.Client().Do(req)
	if err == nil {
		io.Copy(io.Discard, response.Body)
		response.Body.Close()
		t.Fatalf("socket drop returned HTTP %d", response.StatusCode)
	}
}
