package auth

import (
	"bytes"
	"fmt"
	"gowave/internal/database"
	"gowave/internal/testdb"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"gorm.io/gorm"
)

const registerSecret = "private-secret-at-least-32-characters"

func countAdmins(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var count int64
	if err := db.Model(&database.Admin{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	return count
}

func TestRegisterConcurrentCreatesOneAdmin(t *testing.T) {
	db := testdb.Open(t)
	h := NewHandler(db, registerSecret, 12)
	codes := make([]int, 8)
	var wg sync.WaitGroup
	for i := range codes {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			body := fmt.Sprintf(`{"username":"admin%d","password":"password-for-tests"}`, i)
			w := httptest.NewRecorder()
			h.Register(w, httptest.NewRequest("POST", "/api/auth/register", bytes.NewBufferString(body)))
			codes[i] = w.Code
		}(i)
	}
	wg.Wait()
	ok := 0
	for _, code := range codes {
		switch code {
		case http.StatusOK:
			ok++
		case http.StatusForbidden:
		default:
			t.Fatalf("unexpected status %d in %v", code, codes)
		}
	}
	if ok != 1 || countAdmins(t, db) != 1 {
		t.Fatalf("want exactly one admin, got %d successes and %d admins", ok, countAdmins(t, db))
	}
}

// ownerFirstBody simulates an attacker whose request passed the initial check
// but whose body arrives only after the legitimate owner has registered.
type ownerFirstBody struct {
	db   *gorm.DB
	body io.Reader
	once sync.Once
}

func (b *ownerFirstBody) Read(p []byte) (int, error) {
	b.once.Do(func() {
		b.db.Create(&database.Admin{UUID: "owner", Username: "owner", Role: "ADMIN"})
	})
	return b.body.Read(p)
}

func TestRegisterRechecksAfterReadingBody(t *testing.T) {
	db := testdb.Open(t)
	body := &ownerFirstBody{db: db, body: bytes.NewBufferString(`{"username":"attacker","password":"password-for-tests"}`)}
	w := httptest.NewRecorder()
	NewHandler(db, registerSecret, 12).Register(w, httptest.NewRequest("POST", "/api/auth/register", body))
	if w.Code != http.StatusForbidden {
		t.Fatalf("late registration returned %d: %s", w.Code, w.Body.String())
	}
	if countAdmins(t, db) != 1 {
		t.Fatalf("second admin was created")
	}
}

func TestRegisterRejectsOversizedBody(t *testing.T) {
	db := testdb.Open(t)
	body := `{"username":"admin","password":"` + string(bytes.Repeat([]byte("a"), 8192)) + `"}`
	w := httptest.NewRecorder()
	NewHandler(db, registerSecret, 12).Register(w, httptest.NewRequest("POST", "/api/auth/register", bytes.NewBufferString(body)))
	if w.Code != http.StatusBadRequest || countAdmins(t, db) != 0 {
		t.Fatalf("oversized body returned %d", w.Code)
	}
}
