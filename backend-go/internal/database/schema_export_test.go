package database

import (
	"context"
	"github.com/glebarez/sqlite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"os"
	"strings"
	"testing"
	"time"
)

// Regenerate only when changing the pre-release baseline schema:
// GOWAVE_EXPORT_SCHEMA=1 go test ./internal/database -run TestExportSchema
type schemaLogger struct {
	logger.Interface
	statements []string
}

func (l *schemaLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	sql, _ := fc()
	if strings.HasPrefix(sql, "CREATE ") {
		l.statements = append(l.statements, sql)
	}
}
func TestExportSchema(t *testing.T) {
	if os.Getenv("GOWAVE_EXPORT_SCHEMA") != "1" {
		t.Skip("explicit schema export only")
	}
	for _, dialect := range []string{"sqlite", "postgres"} {
		l := &schemaLogger{Interface: logger.Discard}
		var d gorm.Dialector = sqlite.Open(":memory:")
		if dialect == "postgres" {
			d = postgres.Open("host=/tmp/gowave-schema-no-server user=unused dbname=unused sslmode=disable")
		}
		db, err := gorm.Open(d, &gorm.Config{DryRun: true, DisableAutomaticPing: true, Logger: l})
		if err != nil {
			t.Fatal(err)
		}
		if err = db.AutoMigrate(schemaModels()...); err != nil {
			t.Fatal(err)
		}
		var output strings.Builder
		output.WriteString("-- Gowave pre-release schema baseline; see schema_export_test.go.\n")
		for _, sql := range l.statements {
			sql = strings.Replace(sql, "CREATE TABLE ", "CREATE TABLE IF NOT EXISTS ", 1)
			if !strings.Contains(sql, "INDEX IF NOT EXISTS") {
				sql = strings.Replace(sql, "CREATE UNIQUE INDEX ", "CREATE UNIQUE INDEX IF NOT EXISTS ", 1)
			}
			if !strings.Contains(sql, "INDEX IF NOT EXISTS") {
				sql = strings.Replace(sql, "CREATE INDEX ", "CREATE INDEX IF NOT EXISTS ", 1)
			}
			output.WriteString(sql)
			output.WriteString(";\n")
		}
		if err = os.MkdirAll("migrations/"+dialect, 0755); err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile("migrations/"+dialect+"/001_initial.sql", []byte(output.String()), 0644); err != nil {
			t.Fatal(err)
		}
	}
}
