package subscription

import (
	"context"
	"log"
	"sync"
	"time"

	"gowave/internal/database"

	"gorm.io/gorm"
)

const (
	// historyKeep is how many requests per user upstream keeps.
	historyKeep = 24
	// historyQueue bounds memory; a full queue falls back to writing inline.
	historyQueue = 4096
	historyBatch = 500
)

// historyWriter takes subscription request history off the response path,
// like upstream's ADD_SUBSCRIPTION_REQUEST_RECORD queue job: requests enqueue
// a row and a single goroutine inserts and trims them in batches.
type historyWriter struct {
	db      *gorm.DB
	queue   chan database.UserSubscriptionRequestHistory
	mu      sync.RWMutex
	running bool
	stopped chan struct{}
}

func newHistoryWriter(db *gorm.DB) *historyWriter {
	return &historyWriter{db: db, queue: make(chan database.UserSubscriptionRequestHistory, historyQueue)}
}

// add queues a row, or writes it inline when no writer runs (tests, CLI) or
// the queue is full, so history is never dropped.
func (w *historyWriter) add(record database.UserSubscriptionRequestHistory) {
	w.mu.RLock()
	if w.running {
		select {
		case w.queue <- record:
			w.mu.RUnlock()
			return
		default:
		}
	}
	w.mu.RUnlock()
	w.write([]database.UserSubscriptionRequestHistory{record})
}

// Run writes queued rows until ctx ends, then drains what is left.
func (w *historyWriter) Run(ctx context.Context) {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return
	}
	w.running = true
	w.stopped = make(chan struct{})
	w.mu.Unlock()
	defer close(w.stopped)

	batch := make([]database.UserSubscriptionRequestHistory, 0, historyBatch)
	for {
		select {
		case record := <-w.queue:
			batch = append(batch[:0], record)
			batch = w.collect(batch)
			w.write(batch)
		case <-ctx.Done():
			w.mu.Lock()
			w.running = false
			w.mu.Unlock()
			// add no longer queues; flush everything accepted so far.
			for {
				batch = w.collect(batch[:0])
				if len(batch) == 0 {
					return
				}
				w.write(batch)
			}
		}
	}
}

func (w *historyWriter) isRunning() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.running
}

// Wait blocks until Run has drained the queue after its context ended.
func (w *historyWriter) Wait(timeout time.Duration) {
	w.mu.RLock()
	stopped := w.stopped
	w.mu.RUnlock()
	if stopped == nil {
		return
	}
	select {
	case <-stopped:
	case <-time.After(timeout):
		log.Printf("[SUB] subscription history: %d rows not written before shutdown", len(w.queue))
	}
}

func (w *historyWriter) collect(batch []database.UserSubscriptionRequestHistory) []database.UserSubscriptionRequestHistory {
	for len(batch) < historyBatch {
		select {
		case record := <-w.queue:
			batch = append(batch, record)
		default:
			return batch
		}
	}
	return batch
}

func (w *historyWriter) write(batch []database.UserSubscriptionRequestHistory) {
	if len(batch) == 0 {
		return
	}
	if err := w.db.CreateInBatches(batch, historyBatch).Error; err != nil {
		log.Printf("[SUB] subscription history insert: %v", err)
		return
	}
	seen := make(map[uint64]bool, len(batch))
	users := make([]uint64, 0, len(batch))
	for _, record := range batch {
		if !seen[record.UserID] {
			seen[record.UserID] = true
			users = append(users, record.UserID)
		}
	}
	// One statement trims every user of the batch to its latest rows.
	if err := w.db.Exec(`DELETE FROM user_subscription_request_history WHERE id IN (
		SELECT id FROM (
			SELECT id, ROW_NUMBER() OVER (PARTITION BY user_id ORDER BY request_at DESC, id DESC) AS position
			FROM user_subscription_request_history WHERE user_id IN ?
		) ranked WHERE position > ?)`, users, historyKeep).Error; err != nil {
		log.Printf("[SUB] subscription history trim: %v", err)
	}
}
