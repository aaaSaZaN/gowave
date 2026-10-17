package database

import (
	"strings"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func Init(driver string, databaseURL string) (*gorm.DB, error) {
	var dialector gorm.Dialector

	isPostgres := strings.ToLower(driver) == "postgres" || strings.HasPrefix(databaseURL, "postgres://") || strings.HasPrefix(databaseURL, "postgresql://")
	if isPostgres {
		dialector = postgres.Open(databaseURL)
	} else {
		dialector = sqlite.Open(databaseURL)
	}

	db, err := gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
		// Timestamps are stored in UTC so SQLite's text comparison orders them
		// correctly and API output matches upstream's ISO "Z" strings.
		NowFunc: func() time.Time { return time.Now().UTC() },
	})
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err == nil {
		if !isPostgres {
			// SQLite in concurrent environment: limit to 1 open connection to avoid database is locked
			sqlDB.SetMaxOpenConns(1)
			sqlDB.SetMaxIdleConns(1)
			if err := db.Exec("PRAGMA journal_mode = WAL").Error; err != nil {
				return nil, err
			}
			if err := db.Exec("PRAGMA busy_timeout = 5000").Error; err != nil {
				return nil, err
			}
			if err := db.Exec("PRAGMA synchronous = NORMAL").Error; err != nil {
				return nil, err
			}
			if err := db.Exec("PRAGMA foreign_keys = ON").Error; err != nil {
				return nil, err
			}
		} else {
			SetMaxConnections(db, DefaultMaxConnections)
			sqlDB.SetConnMaxIdleTime(10 * time.Minute)
			sqlDB.SetConnMaxLifetime(time.Hour)
		}
	}

	return db, nil
}

// DefaultMaxConnections is the PostgreSQL pool size unless DB_MAX_CONNECTIONS
// overrides it.
const DefaultMaxConnections = 20

// SetMaxConnections sizes the PostgreSQL pool. Idle connections are kept up to
// the same limit: with fewer, bursts above the idle limit close connections and
// the next burst makes PostgreSQL fork new backends (thousands per minute under
// load). Each kept backend costs a few MB, so the limit also bounds memory.
func SetMaxConnections(db *gorm.DB, n int) {
	sqlDB, err := db.DB()
	if err != nil || n < 1 || db.Dialector.Name() != "postgres" {
		return
	}
	sqlDB.SetMaxOpenConns(n)
	sqlDB.SetMaxIdleConns(n)
}
