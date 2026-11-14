package auth

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"gowave/internal/database"
)

func TestRemovedYandexCannotBeEnabledBySavedSettings(t *testing.T) {
	db := securityDB(t)
	settings := `{"yandex":{"enabled":true,"clientId":"old-client","clientSecret":"old-secret","allowedEmails":["admin@example.com"]},"github":{"enabled":true,"clientId":"github-client","clientSecret":"github-secret"}}`
	if err := db.Model(&database.PanelSetting{}).Where("id=1").Update("oauth2_settings", settings).Error; err != nil {
		t.Fatal(err)
	}

	status := httptest.NewRecorder()
	NewHandler(db, "private-secret-at-least-32-characters", 12).GetStatus(status, httptest.NewRequest("GET", "/api/auth/status", nil))
	var body struct {
		Response struct {
			Authentication struct {
				OAuth2 struct {
					Providers map[string]bool `json:"providers"`
				} `json:"oauth2"`
			} `json:"authentication"`
		} `json:"response"`
	}
	if err := json.Unmarshal(status.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	providers := body.Response.Authentication.OAuth2.Providers
	if providers["yandex"] || !providers["github"] {
		t.Fatalf("unexpected providers: %v", providers)
	}

	handler := NewOAuthHandler(db, "private-secret-at-least-32-characters", 12)
	if _, _, _, err := handler.configuration("github"); err != nil {
		t.Fatalf("GitHub disabled: %v", err)
	}
	for _, endpoint := range []string{"authorize", "callback"} {
		t.Run(endpoint, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodPost, "/api/auth/oauth2/"+endpoint, bytes.NewBufferString(`{"provider":"yandex","code":"old-code","state":"old-state"}`))
			if endpoint == "authorize" {
				handler.Authorize(w, r)
			} else {
				handler.Callback(w, r)
			}
			if w.Code < 400 {
				t.Fatalf("removed provider accepted: %d %s", w.Code, w.Body.String())
			}
		})
	}
}
