package auth

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"remnawave-go/config"
	"remnawave-go/internal/database"
	"remnawave-go/internal/middleware"
	"remnawave-go/internal/notify"
	"testing"
	"time"

	"github.com/fxamacker/cbor/v2"
	"gorm.io/gorm"
)

func securityDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := database.Init("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if err = database.AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sql, _ := db.DB(); sql.Close() })
	hash, _ := HashPassword("password-for-tests", "private-secret-at-least-32-characters")
	if err = db.Create(&database.Admin{UUID: "test-admin", Username: "admin", Role: "ADMIN", PasswordHash: hash}).Error; err != nil {
		t.Fatal(err)
	}
	return db
}

func TestPublicPasskeyBypassRejected(t *testing.T) {
	db := securityDB(t)
	h := NewPasskeyHandler(db, "private-secret-at-least-32-characters", 12)
	for _, body := range []string{`{}`, `invalid`, `{"response":{"id":"unknown"}}`} {
		w := httptest.NewRecorder()
		h.VerifyAuthentication(w, httptest.NewRequest("POST", "/", bytes.NewBufferString(body)))
		if w.Code < 400 {
			t.Fatalf("accepted invalid assertion: %d %s", w.Code, w.Body.String())
		}
	}
}

func TestPasswordAuthenticationPolicy(t *testing.T) {
	db := securityDB(t)
	for _, tc := range []struct {
		name, settings string
		enabled        bool
		status         int
	}{
		{"missing enabled preserves true", `{"other":true}`, true, http.StatusOK},
		{"missing enabled preserves false", `{"other":true}`, false, http.StatusForbidden},
		{"explicit false", `{"enabled":false}`, true, http.StatusForbidden},
		{"explicit true", `{"enabled":true}`, false, http.StatusOK},
		{"invalid JSON", `{"enabled":`, true, http.StatusInternalServerError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := db.Model(&database.RemnawaveSetting{}).Where("id=1").Updates(map[string]interface{}{"password_settings": tc.settings, "password_auth_enabled": tc.enabled}).Error; err != nil {
				t.Fatal(err)
			}
			w := httptest.NewRecorder()
			NewHandler(db, "private-secret-at-least-32-characters", 12).Login(w, httptest.NewRequest("POST", "/", bytes.NewBufferString(`{"username":"admin","password":"password-for-tests"}`)))
			if w.Code != tc.status {
				t.Fatalf("login returned %d: %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestFailedLoginEventOmitsPassword(t *testing.T) {
	db := securityDB(t)
	notifier, err := notify.New(&config.Config{Notifications: config.NotificationsConfig{WebhookEnabled: true, WebhookURLs: []string{"https://example.com/webhook"}}})
	if err != nil {
		t.Fatal(err)
	}
	notify.SetDefault(notifier)
	t.Cleanup(func() { notify.SetDefault(nil) })
	loginAttempt(db, false, "admin", "submitted-secret", "127.0.0.1", "test-agent", "Invalid password.")
	var job database.Job
	if err := db.Where("entity_id = ?", "service.login_attempt_failed").First(&job).Error; err != nil {
		t.Fatal(err)
	}
	var delivery struct {
		Payload string `json:"payload"`
	}
	if err := json.Unmarshal([]byte(job.Payload), &delivery); err != nil {
		t.Fatal(err)
	}
	var event struct {
		Data struct {
			Attempt map[string]string `json:"loginAttempt"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(delivery.Payload), &event); err != nil {
		t.Fatal(err)
	}
	if _, present := event.Data.Attempt["password"]; present {
		t.Fatal("password present in failed-login event")
	}
	for key, want := range map[string]string{"username": "admin", "ip": "127.0.0.1", "userAgent": "test-agent", "description": "Invalid password."} {
		if event.Data.Attempt[key] != want {
			t.Errorf("%s = %q, want %q", key, event.Data.Attempt[key], want)
		}
	}
}

func TestChallengeExpiryAndConsumption(t *testing.T) {
	db := securityDB(t)
	for _, record := range []database.AuthChallenge{{ID: "valid", Purpose: "test", AdminUUID: "test-admin", ExpiresAt: time.Now().Add(time.Minute)}, {ID: "expired", Purpose: "test", ExpiresAt: time.Now().Add(-time.Second)}} {
		if err := db.Create(&record).Error; err != nil {
			t.Fatal(err)
		}
	}
	if _, err := ConsumeChallenge(db, "valid", "test", "wrong-admin"); err == nil {
		t.Fatal("accepted wrong owner")
	}
	if _, err := ConsumeChallenge(db, "valid", "test", "test-admin"); err != nil {
		t.Fatal(err)
	}
	if _, err := ConsumeChallenge(db, "valid", "test", "test-admin"); err == nil {
		t.Fatal("accepted replay")
	}
	if _, err := ConsumeChallenge(db, "expired", "test", ""); err == nil {
		t.Fatal("accepted expired challenge")
	}
}

func TestPasskeyRegistrationAndSignedLogin(t *testing.T) {
	db := securityDB(t)
	db.Model(&database.RemnawaveSetting{}).Where("id=1").Update("passkey_settings", `{"enabled":true,"rpId":"panel.example","origin":"https://panel.example"}`)
	h := NewPasskeyHandler(db, "private-secret-at-least-32-characters", 12)
	adminRequest := func(body []byte) *http.Request {
		r := httptest.NewRequest("POST", "/", bytes.NewReader(body))
		return r.WithContext(context.WithValue(r.Context(), middleware.AdminContextKey, &middleware.TokenClaims{UUID: "test-admin", Role: "ADMIN"}))
	}
	w := httptest.NewRecorder()
	h.RegistrationOptions(w, adminRequest(nil))
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	var options struct {
		Response struct {
			Challenge string `json:"challenge"`
		} `json:"response"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &options); err != nil {
		t.Fatal(err)
	}
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	credentialID := []byte("test-credential-id")
	rpHash := sha256.Sum256([]byte("panel.example"))
	cose, _ := cbor.Marshal(map[int]interface{}{1: 2, 3: -7, -1: 1, -2: key.X.FillBytes(make([]byte, 32)), -3: key.Y.FillBytes(make([]byte, 32))})
	data := append([]byte{}, rpHash[:]...)
	data = append(data, 0x45, 0, 0, 0, 0)
	data = append(data, make([]byte, 16)...)
	data = binary.BigEndian.AppendUint16(data, uint16(len(credentialID)))
	data = append(data, credentialID...)
	data = append(data, cose...)
	attestation, _ := cbor.Marshal(map[string]interface{}{"fmt": "none", "attStmt": map[string]interface{}{}, "authData": data})
	client, _ := json.Marshal(map[string]string{"type": "webauthn.create", "challenge": options.Response.Challenge, "origin": "https://panel.example"})
	encode := base64.RawURLEncoding.EncodeToString
	creation, _ := json.Marshal(map[string]interface{}{"response": map[string]interface{}{"id": encode(credentialID), "rawId": encode(credentialID), "type": "public-key", "response": map[string]interface{}{"clientDataJSON": encode(client), "attestationObject": encode(attestation), "transports": []string{"internal"}}}})
	w = httptest.NewRecorder()
	h.VerifyRegistration(w, adminRequest(creation))
	if w.Code != 200 {
		t.Fatalf("registration: %d %s", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	h.AuthenticationOptions(w, httptest.NewRequest("GET", "/", nil))
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	json.Unmarshal(w.Body.Bytes(), &options)
	client, _ = json.Marshal(map[string]string{"type": "webauthn.get", "challenge": options.Response.Challenge, "origin": "https://panel.example"})
	data = append([]byte{}, rpHash[:]...)
	data = append(data, 0x05, 0, 0, 0, 1)
	clientHash := sha256.Sum256(client)
	signedHash := sha256.Sum256(append(append([]byte{}, data...), clientHash[:]...))
	signature, _ := ecdsa.SignASN1(rand.Reader, key, signedHash[:])
	assertion, _ := json.Marshal(map[string]interface{}{"response": map[string]interface{}{"id": encode(credentialID), "rawId": encode(credentialID), "type": "public-key", "response": map[string]string{"clientDataJSON": encode(client), "authenticatorData": encode(data), "signature": encode(signature), "userHandle": encode([]byte("test-admin"))}}})
	w = httptest.NewRecorder()
	h.VerifyAuthentication(w, httptest.NewRequest("POST", "/", bytes.NewReader(assertion)))
	if w.Code != 200 {
		t.Fatalf("login: %d %s", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	h.VerifyAuthentication(w, httptest.NewRequest("POST", "/", bytes.NewReader(assertion)))
	if w.Code != 401 {
		t.Fatalf("replay returned %d", w.Code)
	}
}
