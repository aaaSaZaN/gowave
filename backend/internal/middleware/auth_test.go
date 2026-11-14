package middleware_test

import (
	"encoding/json"
	"github.com/go-chi/chi/v5"
	"gowave/internal/auth"
	"gowave/internal/database"
	"gowave/internal/middleware"
	"gowave/internal/tokens"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAuthorizationAndOtt(t *testing.T) {
	db, err := database.Init("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	sql, _ := db.DB()
	defer sql.Close()
	if err := database.AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	secret := "private-test-secret-at-least-32-characters"
	db.Create(&database.Admin{UUID: "admin", Username: "admin", Role: "ADMIN"})
	db.Create(&database.ApiToken{UUID: "api", Name: "read", Scopes: database.StringArray{"users:read"}, ExpireAt: time.Now().UTC().Add(time.Hour)})
	router := chi.NewRouter()
	router.Use(middleware.Auth(secret, db))
	handler := func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) }
	router.Get("/api/users", handler)
	router.Get("/api/subscriptions/by-short-uuid/{shortUuid}/raw", handler)
	router.Delete("/api/users/{userId}", handler)
	router.Post("/api/tokens/ott", tokens.NewHandler(db, secret).GetOtt)
	request := func(method, path, token string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(method, path, nil)
		r.Header.Set("Authorization", "Bearer "+token)
		router.ServeHTTP(w, r)
		return w
	}
	api, _ := auth.GenerateToken("api", "", "API", secret, 1)
	if code := request("GET", "/api/users", api).Code; code != 204 {
		t.Fatalf("read token rejected %d", code)
	}
	for _, path := range []string{"/api/users/1", "/api/users/by-username/test"} {
		if code := request("DELETE", path, api).Code; code != 403 {
			t.Fatalf("write admitted %s: %d", path, code)
		}
	}
	if code := request("POST", "/api/tokens/ott", api).Code; code != 403 {
		t.Fatalf("API OTT %d", code)
	}
	db.Model(&database.ApiToken{}).Where("uuid='api'").Update("scopes", database.StringArray{"*"})
	if code := request("POST", "/api/tokens/ott", api).Code; code != 403 {
		t.Fatalf("wildcard API OTT %d", code)
	}
	for _, scope := range []string{"subscription:raw", "subscriptions:raw", "users:read"} {
		if err := db.Model(&database.ApiToken{}).Where("uuid='api'").Update("scopes", database.StringArray{scope}).Error; err != nil {
			t.Fatal(err)
		}
		want := http.StatusNoContent
		if scope == "users:read" {
			want = http.StatusForbidden
		}
		if code := request("GET", "/api/subscriptions/by-short-uuid/test/raw", api).Code; code != want {
			t.Fatalf("scope %s returned %d, want %d", scope, code, want)
		}
	}
	missing, _ := auth.GenerateToken("missing", "", "ADMIN", secret, 1)
	if code := request("GET", "/api/users", missing).Code; code != 401 {
		t.Fatal("nonexistent administrator admitted")
	}
	admin, _ := auth.GenerateToken("admin", "", "ADMIN", secret, 1)
	w := request("POST", "/api/tokens/ott", admin)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	var output struct {
		Response struct {
			OTT string `json:"ott"`
		} `json:"response"`
	}
	json.Unmarshal(w.Body.Bytes(), &output)
	if code := request("DELETE", "/api/users/1", output.Response.OTT).Code; code != 401 {
		t.Fatal("OTT admitted to normal API")
	}
	db.Delete(&database.ApiToken{}, "uuid='api'")
	if code := request("GET", "/api/users", api).Code; code != 401 {
		t.Fatal("revoked API token admitted")
	}
}
