// Package jobs provides a transactional outbox and bounded, leased workers.
package jobs

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gowave/internal/database"
	"log"
	"strconv"
	"sync"
	"time"
)

type Handler func(context.Context, *database.Job) error

// RetryAfterError asks the queue to retry no earlier than Delay, for example
// when a remote API answers 429 with Retry-After.
type RetryAfterError struct {
	Err   error
	Delay time.Duration
}

func (e *RetryAfterError) Error() string { return e.Err.Error() }
func (e *RetryAfterError) Unwrap() error { return e.Err }

type Queue struct {
	db       *gorm.DB
	handlers map[string]Handler
}

func New(db *gorm.DB) *Queue { return &Queue{db: db, handlers: make(map[string]Handler)} }

// Register is called before Start; handlers remain immutable while workers run.
func (q *Queue) Register(kind string, handler Handler) { q.handlers[kind] = handler }

func Enqueue(tx *gorm.DB, kind, entity, target string, revision uint64, payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	digest := sha256.Sum256([]byte(fmt.Sprintf("%s/%s/%s/%d", kind, entity, target, revision)))
	now := time.Now().UTC()
	record := database.Job{ID: hex.EncodeToString(digest[:]), Kind: kind, EntityID: entity, Target: target, Revision: revision, Payload: string(data), State: "pending", AvailableAt: now, CreatedAt: now, UpdatedAt: now}
	return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&record).Error
}

func ID(kind, entity, target string, revision uint64) string {
	digest := sha256.Sum256([]byte(fmt.Sprintf("%s/%s/%s/%d", kind, entity, target, revision)))
	return hex.EncodeToString(digest[:])
}

type UserSync struct {
	ID           uint64 `json:"id"`
	VlessUUID    string `json:"vlessUuid"`
	PreviousUUID string `json:"previousUuid"`
	Deleted      bool   `json:"deleted"`
}

func EnqueueUser(tx *gorm.DB, user *database.User, previous *string, deleted bool) error {
	var targets []string
	if err := tx.Model(&database.Node{}).Where("is_disabled = ?", false).Pluck("uuid", &targets).Error; err != nil {
		return err
	}
	payload := UserSync{ID: user.ID, VlessUUID: user.VlessUUID, Deleted: deleted}
	if previous != nil {
		payload.PreviousUUID = *previous
	}
	revision := user.Revision
	if revision == 0 {
		revision = uint64(user.UpdatedAt.UnixNano())
	}
	for _, target := range targets {
		if err := Enqueue(tx, "user-sync", strconv.FormatUint(user.ID, 10), target, revision, payload); err != nil {
			return err
		}
	}
	return nil
}

func EnqueueNode(tx *gorm.DB, nodeUUID, kind string) error {
	return Enqueue(tx, kind, nodeUUID, nodeUUID, uint64(time.Now().UnixNano()), map[string]string{"uuid": nodeUUID})
}

const (
	completedRetention = 24 * time.Hour
	failedRetention    = 7 * 24 * time.Hour
)

// Cleanup removes terminal jobs based on their last state change. Pending and
// running jobs are retained regardless of age.
func Cleanup(tx *gorm.DB, now time.Time) error {
	return tx.Where("(state = ? AND updated_at < ?) OR (state = ? AND updated_at < ?)",
		"completed", now.UTC().Add(-completedRetention), "failed", now.UTC().Add(-failedRetention)).
		Delete(&database.Job{}).Error
}

func (q *Queue) claim() (*database.Job, error) {
	var job database.Job
	now := time.Now().UTC()
	err := q.db.Transaction(func(tx *gorm.DB) error {
		query := tx.Model(&database.Job{}).Where("available_at <= ? AND (state = 'pending' OR (state = 'running' AND lease_until <= ?))", now, now).
			Where("NOT EXISTS (SELECT 1 FROM jobs busy WHERE busy.target = jobs.target AND busy.id <> jobs.id AND busy.state = 'running' AND busy.lease_until > ?)", now).
			Order("created_at ASC, id ASC")
		if tx.Dialector.Name() == "postgres" {
			query = query.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"})
		}
		if err := query.First(&job).Error; err != nil {
			return err
		}
		if tx.Dialector.Name() == "postgres" {
			if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtext(?))", job.Target).Error; err != nil {
				return err
			}
			var busy int64
			if err := tx.Model(&database.Job{}).Where("target = ? AND id <> ? AND state = 'running' AND lease_until > ?", job.Target, job.ID, now).Count(&busy).Error; err != nil {
				return err
			}
			if busy > 0 {
				return gorm.ErrRecordNotFound
			}
		}
		lease := now.Add(2 * time.Minute)
		result := tx.Model(&database.Job{}).Where("id = ? AND (state = 'pending' OR (state = 'running' AND lease_until <= ?))", job.ID, now).Updates(map[string]interface{}{"state": "running", "lease_until": lease, "attempts": gorm.Expr("attempts + 1"), "updated_at": now})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}
		job.Attempts++
		job.State = "running"
		job.LeaseUntil = &lease
		return nil
	})
	return &job, err
}

func (q *Queue) RunOne(ctx context.Context) (bool, error) {
	job, err := q.claim()
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	handler, ok := q.handlers[job.Kind]
	if !ok {
		err = fmt.Errorf("unsupported job kind %s", job.Kind)
	} else {
		func() {
			defer func() {
				if value := recover(); value != nil {
					err = fmt.Errorf("worker panic: %v", value)
				}
			}()
			workCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
			defer cancel()
			err = handler(workCtx, job)
		}()
	}
	now := time.Now().UTC()
	updates := map[string]interface{}{"state": "completed", "lease_until": nil, "last_error": "", "updated_at": now}
	if err != nil {
		state := "pending"
		if job.Attempts >= 10 {
			state = "failed"
		}
		shift := job.Attempts
		if shift > 9 {
			shift = 9
		}
		delay := time.Duration(1<<shift) * time.Second
		if delay > 5*time.Minute {
			delay = 5 * time.Minute
		}
		var retryAfter *RetryAfterError
		if errors.As(err, &retryAfter) && retryAfter.Delay > delay {
			delay = retryAfter.Delay
		}
		updates["state"], updates["last_error"], updates["available_at"] = state, err.Error(), now.Add(delay)
	}
	result := q.db.Model(&database.Job{}).Where("id = ? AND state = 'running' AND lease_until = ?", job.ID, *job.LeaseUntil).Updates(updates)
	if result.Error != nil {
		return true, result.Error
	}
	return true, err
}

func (q *Queue) Start(ctx context.Context) {
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			timer := time.NewTicker(time.Second)
			defer timer.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}
				worked, err := q.RunOne(ctx)
				if err != nil {
					log.Printf("[JOBS] %v", err)
				}
				if worked {
					continue
				}
				select {
				case <-ctx.Done():
					return
				case <-timer.C:
				}
			}
		}()
	}
	wg.Wait()
}
