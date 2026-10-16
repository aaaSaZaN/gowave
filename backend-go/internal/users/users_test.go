package users

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"gowave/internal/database"

	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := database.Init("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite in-memory db: %v", err)
	}
	sql, _ := db.DB()
	t.Cleanup(func() { sql.Close() })
	err = database.AutoMigrate(db)
	if err != nil {
		t.Fatalf("failed to auto-migrate: %v", err)
	}
	return db
}

func TestGetStreamTelegramIdFilter(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db)

	tg1 := int64(111111)
	tg2 := int64(222222)

	_, err := svc.Create(CreateUserDTO{
		Username:   "user1",
		TelegramID: &tg1,
		ExpireAt:   time.Now().Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("failed to create user1: %v", err)
	}

	_, err = svc.Create(CreateUserDTO{
		Username:   "user2",
		TelegramID: &tg2,
		ExpireAt:   time.Now().Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("failed to create user2: %v", err)
	}

	users, _, hasMore, err := svc.GetStream(StreamQuery{TelegramID: &tg1})
	if err != nil {
		t.Fatalf("failed to get stream: %v", err)
	}
	if len(users) != 1 {
		t.Fatalf("expected 1 user, got %d", len(users))
	}
	if users[0].Username != "user1" {
		t.Errorf("expected user1, got %s", users[0].Username)
	}
	if hasMore {
		t.Errorf("expected hasMore=false")
	}
}

func TestActiveInternalSquads(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db)

	squad := database.InternalSquad{
		UUID: "squad-uuid-123",
		Name: "Premium Squad",
	}
	if err := db.Create(&squad).Error; err != nil {
		t.Fatalf("failed to create squad: %v", err)
	}

	user, err := svc.Create(CreateUserDTO{
		Username:             "squad_user",
		ExpireAt:             time.Now().Add(24 * time.Hour),
		ActiveInternalSquads: []string{"squad-uuid-123"},
	})
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	if len(user.ActiveInternalSquads) != 1 {
		t.Fatalf("expected 1 active squad, got %d", len(user.ActiveInternalSquads))
	}
	if user.ActiveInternalSquads[0].UUID != "squad-uuid-123" {
		t.Errorf("expected squad-uuid-123, got %s", user.ActiveInternalSquads[0].UUID)
	}

	formatted := FormatUser(user, "sub.example.com")
	squadsList, ok := formatted["activeInternalSquads"].([]map[string]interface{})
	if !ok {
		t.Fatalf("expected activeInternalSquads to be []map[string]interface{}, got %T", formatted["activeInternalSquads"])
	}
	if len(squadsList) != 1 || squadsList[0]["uuid"] != "squad-uuid-123" {
		t.Errorf("unexpected formatted squad: %+v", squadsList)
	}
}

func TestBulkExtendExpirationDate(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db)

	initialExpire := time.Now().Add(-10 * time.Hour)
	u, err := svc.Create(CreateUserDTO{
		Username: "expired_user",
		Status:   "EXPIRED",
		ExpireAt: initialExpire,
	})
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	err = svc.BulkExtendExpirationDate([]uint64{u.ID}, 30)
	if err != nil {
		t.Fatalf("failed to bulk extend: %v", err)
	}

	updated, err := svc.GetByID(u.ID)
	if err != nil {
		t.Fatalf("failed to get user: %v", err)
	}

	if updated.Status != "ACTIVE" {
		t.Errorf("expected status ACTIVE, got %s", updated.Status)
	}
	expectedMin := initialExpire.Add(29 * 24 * time.Hour)
	if updated.ExpireAt.Before(expectedMin) {
		t.Errorf("expected expireAt after %v, got %v", expectedMin, updated.ExpireAt)
	}
}

func TestHandlersIntegration(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(db)
	h := NewHandler(svc, "sub.example.com")

	tg := int64(999888)
	u, err := svc.Create(CreateUserDTO{
		Username:   "botuser",
		TelegramID: &tg,
		ExpireAt:   time.Now().Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/users/stream?telegramId=999888&size=250", nil)
	w := httptest.NewRecorder()
	h.GetUsersStream(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GetUsersStream returned %d", w.Code)
	}

	body := []byte(`{"userIds":[` + strconv.FormatUint(u.ID, 10) + `],"extendDays":30}`)
	req = httptest.NewRequest("POST", "/api/users/bulk/extend-expiration-date", bytes.NewReader(body))
	w = httptest.NewRecorder()
	h.BulkExtendExpirationDate(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("BulkExtendExpirationDate returned %d", w.Code)
	}

	body = []byte(`{"userIds":[` + strconv.FormatUint(u.ID, 10) + `],"fields":{"tag":"VIP"}}`)
	req = httptest.NewRequest("POST", "/api/users/bulk/update", bytes.NewReader(body))
	w = httptest.NewRecorder()
	h.BulkUpdate(w, req)
	if w.Code != http.StatusAccepted {
		t.Fatalf("BulkUpdate returned %d", w.Code)
	}

	updated, _ := svc.GetByID(u.ID)
	if updated.Tag != "VIP" {
		t.Errorf("expected tag VIP, got %s", updated.Tag)
	}
}
