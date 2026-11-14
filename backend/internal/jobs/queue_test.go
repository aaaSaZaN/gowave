package jobs

import (
	"context"
	"errors"
	"gorm.io/gorm"
	"gowave/internal/database"
	"testing"
	"time"
)

func TestQueueRollbackRetryAndLease(t *testing.T) {
	db, err := database.Init("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	sql, _ := db.DB()
	defer sql.Close()
	if err := database.AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	rollback := errors.New("rollback")
	db.Transaction(func(tx *gorm.DB) error {
		if err := Enqueue(tx, "test", "user", "node", 1, nil); err != nil {
			return err
		}
		return rollback
	})
	var count int64
	db.Model(&database.Job{}).Count(&count)
	if count != 0 {
		t.Fatal("job escaped rolled back transaction")
	}
	Enqueue(db, "test", "user", "node", 1, nil)
	Enqueue(db, "test", "user", "node", 1, nil)
	db.Model(&database.Job{}).Count(&count)
	if count != 1 {
		t.Fatal("duplicate job")
	}
	q := New(db)
	q.Register("test", func(context.Context, *database.Job) error { return errors.New("offline") })
	worked, err := q.RunOne(context.Background())
	if !worked || err == nil {
		t.Fatal("failed job not attempted")
	}
	var job database.Job
	db.First(&job)
	if job.State != "pending" || job.Attempts != 1 || job.LastError == "" || job.LeaseUntil != nil {
		t.Fatalf("retry state %+v", job)
	}
	if worked, _ := q.RunOne(context.Background()); worked {
		t.Fatal("retry ignored backoff")
	}
	db.Model(&job).Updates(map[string]interface{}{"state": "running", "lease_until": time.Now().UTC().Add(-time.Minute), "available_at": time.Now().UTC().Add(-time.Minute)})
	q.Register("test", func(context.Context, *database.Job) error { return nil })
	if worked, err := q.RunOne(context.Background()); !worked || err != nil {
		t.Fatalf("lost lease not recovered %v", err)
	}
	db.First(&job)
	if job.State != "completed" || job.Attempts != 2 {
		t.Fatalf("not completed %+v", job)
	}
}
func TestPerNodeSerialization(t *testing.T) {
	db, err := database.Init("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	sql, _ := db.DB()
	defer sql.Close()
	database.AutoMigrate(db)
	Enqueue(db, "test", "one", "node", 1, nil)
	Enqueue(db, "test", "two", "node", 1, nil)
	Enqueue(db, "test", "other", "other-node", 1, nil)
	q := New(db)
	first, err := q.claim()
	if err != nil {
		t.Fatal(err)
	}
	second, err := q.claim()
	if err != nil {
		t.Fatal(err)
	}
	if first.Target == second.Target {
		t.Fatal("leased two jobs for same node")
	}
	if _, err := q.claim(); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatal("same-node job acquired during active lease", err)
	}
}

func TestPermanentErrorFailsWithoutRetry(t *testing.T) {
	db, err := database.Init("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	sql, _ := db.DB()
	defer sql.Close()
	if err := database.AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	Enqueue(db, "test", "node", "node", 1, nil)
	q := New(db)
	q.Register("test", func(context.Context, *database.Job) error { return Permanent(errors.New("no binary")) })
	if worked, err := q.RunOne(context.Background()); !worked || err == nil {
		t.Fatal("failed job not attempted")
	}
	var job database.Job
	db.First(&job)
	if job.State != "failed" || job.Attempts != 1 || job.LastError != "no binary" {
		t.Fatalf("permanent failure state %+v", job)
	}
}
