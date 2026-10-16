package users

import (
	"gowave/internal/database"
	"testing"
	"time"
)

func TestLifecycleRevokeAndTransactionalOutbox(t *testing.T) {
	db := setupTestDB(t)
	if err := db.Create(&database.Node{UUID: "offline", Name: "offline", Address: "offline.example"}).Error; err != nil {
		t.Fatal(err)
	}
	s := NewService(db)
	u, err := s.Create(CreateUserDTO{Username: "life-user", ExpireAt: time.Now().UTC().Add(24 * time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	old := *u
	var count int64
	db.Model(&database.Job{}).Count(&count)
	if count != 1 {
		t.Fatal("create did not enqueue offline node")
	}
	if err := s.BulkUpdate([]uint64{u.ID}, map[string]interface{}{"status": "LIMITED"}); err != nil {
		t.Fatal(err)
	}
	if err := s.ResetTraffic(u.ID); err != nil {
		t.Fatal(err)
	}
	u, _ = s.GetByID(u.ID)
	if u.Status != "ACTIVE" {
		t.Fatal("reset did not reactivate")
	}
	if _, err := s.Revoke(u.ID, false, ""); err != nil {
		t.Fatal(err)
	}
	u, _ = s.GetByID(u.ID)
	if u.ShortUUID == old.ShortUUID || u.VlessUUID == old.VlessUUID || u.TrojanPassword == old.TrojanPassword || u.SsPassword == old.SsPassword || u.SubRevokedAt == nil || u.Status != "ACTIVE" {
		t.Fatal("revoke did not rotate credentials")
	}
	db.Model(&database.Job{}).Count(&count)
	if count != 4 {
		t.Fatalf("expected four revisions, got %d", count)
	}
	before := u.Revision
	if err := s.BulkUpdateSquads([]uint64{u.ID}, []string{"missing-squad"}); err == nil {
		t.Fatal("accepted missing squad")
	}
	u, _ = s.GetByID(u.ID)
	db.Model(&database.Job{}).Count(&count)
	if u.Revision != before || count != 4 {
		t.Fatal("failed mutation or job not rolled back")
	}
	if err := s.Delete(u.ID); err != nil {
		t.Fatal(err)
	}
	db.Model(&database.Job{}).Count(&count)
	if count != 5 {
		t.Fatal("delete tombstone missing")
	}
}
