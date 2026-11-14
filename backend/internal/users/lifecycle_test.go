package users

import (
	"context"
	"encoding/json"
	"errors"
	"gowave/internal/database"
	"gowave/internal/jobs"
	"testing"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
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

type statementCounter struct {
	logger.Interface
	count int
}

func (c *statementCounter) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	c.count++
}

func TestUpdateLoadsOnceAndKeepsPreviousCredential(t *testing.T) {
	db := setupTestDB(t)
	if err := db.Create(&database.Node{UUID: "offline", Name: "offline", Address: "offline.example"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&database.InternalSquad{UUID: "squad", Name: "Squad"}).Error; err != nil {
		t.Fatal(err)
	}
	created, err := NewService(db).Create(CreateUserDTO{Username: "patched", ExpireAt: time.Now().UTC().Add(24 * time.Hour), ActiveInternalSquads: []string{"squad"}})
	if err != nil {
		t.Fatal(err)
	}
	counter := &statementCounter{Interface: logger.Discard}
	s := NewService(db.Session(&gorm.Session{Logger: counter}))

	updated, err := s.Update(created.ID, map[string]interface{}{"description": "changed"})
	if err != nil {
		t.Fatal(err)
	}
	// UPDATE, the row, traffic, squads, target nodes and the outbox insert.
	if counter.count > 6 {
		t.Fatalf("update issued %d statements, want at most 6", counter.count)
	}
	stored, err := NewService(db).GetByID(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := json.Marshal(FormatUser(updated, "sub.example"))
	want, _ := json.Marshal(FormatUser(stored, "sub.example"))
	if string(got) != string(want) || updated.Description != "changed" || len(updated.ActiveInternalSquads) != 1 {
		t.Fatalf("update returned\n%s\nwant\n%s", got, want)
	}
	previousOf := func(revision uint64) string {
		var job database.Job
		if err := db.Where("kind = ? AND revision = ?", "user-sync", revision).First(&job).Error; err != nil {
			t.Fatal(err)
		}
		var payload jobs.UserSync
		if err := json.Unmarshal([]byte(job.Payload), &payload); err != nil {
			t.Fatal(err)
		}
		return payload.PreviousUUID
	}
	if previous := previousOf(updated.Revision); previous != created.VlessUUID {
		t.Fatalf("unchanged credential: previous %q, want %q", previous, created.VlessUUID)
	}

	rotated, err := s.Update(created.ID, map[string]interface{}{"vlessUuid": "6f1d8c2e-0b7a-4c55-9f3e-2a4b6c8d0e1f"})
	if err != nil {
		t.Fatal(err)
	}
	if previous := previousOf(rotated.Revision); previous != created.VlessUUID || rotated.VlessUUID == created.VlessUUID {
		t.Fatalf("rotated credential: previous %q, want %q", previous, created.VlessUUID)
	}
	if _, err := s.Update(created.ID+100, map[string]interface{}{"description": "missing"}); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("missing user: %v", err)
	}
}
