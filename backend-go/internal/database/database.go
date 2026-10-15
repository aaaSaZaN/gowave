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
			sqlDB.SetMaxOpenConns(50)
			sqlDB.SetMaxIdleConns(10)
			sqlDB.SetConnMaxLifetime(time.Hour)
		}
	}

	return db, nil
}
