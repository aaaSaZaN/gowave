package database

import (
	"context"
	"database/sql"
	"sync"

	"gorm.io/gorm"
)

// commitPool lets write callbacks attach invalidations to the SQL transaction
// that owns the write, including explicit GORM transactions and raw Exec.
type commitPool struct{ *sql.DB }

func (p *commitPool) GetDBConn() (*sql.DB, error) { return p.DB, nil }

func (p *commitPool) BeginTx(ctx context.Context, opts *sql.TxOptions) (gorm.ConnPool, error) {
	tx, err := p.DB.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	return &commitTx{Tx: tx, pool: p}, nil
}

type commitTx struct {
	*sql.Tx
	pool    *commitPool
	mu      sync.Mutex
	actions map[any]func()
}

func (tx *commitTx) GetDBConn() (*sql.DB, error) { return tx.pool.GetDBConn() }

func (tx *commitTx) Commit() error {
	if err := tx.Tx.Commit(); err != nil {
		return err
	}
	tx.mu.Lock()
	actions := tx.actions
	tx.actions = nil
	tx.mu.Unlock()
	for _, action := range actions {
		action()
	}
	return nil
}

// AfterCommit registers a cache invalidation for a successful write callback.
// Repeated registrations with the same key coalesce in the outer transaction.
// An autocommit statement has already committed when its write callback runs.
func AfterCommit(db *gorm.DB, key any, action func()) {
	pool := db.Statement.ConnPool
	if prepared, ok := pool.(*gorm.PreparedStmtTX); ok {
		pool = prepared.Tx
	}
	if tx, ok := pool.(*commitTx); ok {
		tx.mu.Lock()
		if tx.actions == nil {
			tx.actions = make(map[any]func())
		}
		tx.actions[key] = action
		tx.mu.Unlock()
	} else if _, inTransaction := pool.(gorm.TxCommitter); !inTransaction {
		action()
	}
}

const userTrafficChangeKey = "gowave:user_traffic_change"

// UserTrafficChange describes the users affected by a scoped traffic write.
type UserTrafficChange struct{ IDs []uint64 }

func WithUserTrafficChange(db *gorm.DB, ids []uint64) *gorm.DB {
	return db.Session(&gorm.Session{}).Set(userTrafficChangeKey, &UserTrafficChange{IDs: ids})
}

func GetUserTrafficChange(db *gorm.DB) *UserTrafficChange {
	value, _ := db.Get(userTrafficChangeKey)
	change, _ := value.(*UserTrafficChange)
	return change
}
