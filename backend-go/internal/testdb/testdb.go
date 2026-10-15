// Package testdb opens a migrated database for tests: an isolated schema in
// GOWAVE_TEST_POSTGRES when it is set, otherwise in-memory SQLite.
package testdb

import (
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"remnawave-go/internal/database"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Open returns a migrated database that is dropped when the test ends.
func Open(t testing.TB) *gorm.DB {
	t.Helper()
	// Production runs with time.Local = UTC (server.Run).
	time.Local = time.UTC
	dsn := os.Getenv("GOWAVE_TEST_POSTGRES")
	if dsn == "" {
		db, err := database.Init("sqlite", ":memory:")
		if err != nil {
			t.Fatal(err)
		}
		sqlDB, _ := db.DB()
		sqlDB.SetMaxOpenConns(1)
		t.Cleanup(func() { sqlDB.Close() })
		if err := database.AutoMigrate(db); err != nil {
			t.Fatal(err)
		}
		return db
	}
	schema := "t_" + strings.ReplaceAll(uuid.NewString()[:13], "-", "")
	admin, err := database.Init("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	if err := admin.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatal(err)
	}
	adminSQL, _ := admin.DB()
	t.Cleanup(func() {
		admin.Exec("DROP SCHEMA " + schema + " CASCADE")
		adminSQL.Close()
	})
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	query := parsed.Query()
	query.Set("search_path", schema)
	parsed.RawQuery = query.Encode()
	db, err := database.Init("postgres", parsed.String())
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	t.Cleanup(func() { sqlDB.Close() })
	if err := database.AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	return db
}
