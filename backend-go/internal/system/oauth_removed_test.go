package system

import (
	"bytes"
	"encoding/json"
	"gowave/internal/database"
	"gowave/internal/testdb"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSettingsDiscardRemovedOAuthProvider(t *testing.T) {
	db := testdb.Open(t)
	h := NewHandler(db)
	if err := db.Model(&database.RemnawaveSetting{}).Where("id=1").Update("oauth2_settings", `{"yandex":{"enabled":true,"clientId":"old-client","clientSecret":"old-secret"},"github":{"enabled":true}}`).Error; err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	h.GetSettings(response, httptest.NewRequest("GET", "/api/remnawave-settings", nil))
	if strings.Contains(response.Body.String(), "old-secret") || strings.Contains(response.Body.String(), "old-client") {
		t.Fatalf("leaked removed credentials: %s", response.Body.String())
	}
	var body struct {
		Response struct {
			OAuth2 map[string]struct {
				Enabled bool `json:"enabled"`
			} `json:"oauth2Settings"`
		} `json:"response"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Response.OAuth2["yandex"].Enabled || !body.Response.OAuth2["github"].Enabled {
		t.Fatal("unexpected provider settings")
	}
	response = httptest.NewRecorder()
	h.UpdateSettings(response, httptest.NewRequest("PATCH", "/api/remnawave-settings", bytes.NewBufferString(`{"oauth2Settings":{"yandex":{"enabled":true,"clientSecret":"new-secret"},"github":{"enabled":true}}}`)))
	if response.Code >= 400 {
		t.Fatalf("update: %d %s", response.Code, response.Body.String())
	}
	var saved database.RemnawaveSetting
	if err := db.First(&saved).Error; err != nil {
		t.Fatal(err)
	}
	if strings.Contains(saved.OAuth2Settings, "yandex") || !strings.Contains(saved.OAuth2Settings, "github") {
		t.Fatalf("unexpected saved providers: %s", saved.OAuth2Settings)
	}
}
