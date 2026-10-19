package cli

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// The .env lines below are what pasarguard.sh writes for each database.
func TestParsePasarGuardDatabaseURL(t *testing.T) {
	cases := []struct {
		name, raw, driver, dsn string
		ok, fails              bool
	}{
		{name: "sqlite v1 absolute", raw: "sqlite+aiosqlite:////var/lib/pasarguard/db.sqlite3", driver: "sqlite", dsn: "/var/lib/pasarguard/db.sqlite3", ok: true},
		{name: "sqlite v0 absolute", raw: "sqlite:////var/lib/pasarguard/db.sqlite3", driver: "sqlite", dsn: "/var/lib/pasarguard/db.sqlite3", ok: true},
		{name: "sqlite extra slash", raw: "sqlite+aiosqlite://///var/lib/pasarguard/db.sqlite3", driver: "sqlite", dsn: "/var/lib/pasarguard/db.sqlite3", ok: true},
		{name: "sqlite relative", raw: "sqlite+aiosqlite:///db.sqlite3", driver: "sqlite", dsn: "/opt/pasarguard/db.sqlite3", ok: true},
		{name: "postgres via pgbouncer", raw: "postgresql+asyncpg://pasarguard:p%40ss@127.0.0.1:6432/pasarguard", driver: "postgres", dsn: "postgresql://pasarguard:p%40ss@127.0.0.1:6432/pasarguard?default_query_exec_mode=simple_protocol", ok: true},
		{name: "asyncpg ssl option", raw: "postgresql+asyncpg://u:p@127.0.0.1:5432/db?ssl=false", driver: "postgres", dsn: "postgresql://u:p@127.0.0.1:5432/db?default_query_exec_mode=simple_protocol&sslmode=disable", ok: true},
		{name: "plain postgres", raw: "postgres://u:p@localhost/db", driver: "postgres", dsn: "postgresql://u:p@localhost/db?default_query_exec_mode=simple_protocol", ok: true},
		{name: "mysql", raw: "mysql+asyncmy://pasarguard:p@127.0.0.1:3306/pasarguard", fails: true},
		{name: "not a url", raw: "/var/lib/pasarguard/db.sqlite3"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			driver, dsn, ok, err := parsePasarGuardDatabaseURL(tc.raw, "/opt/pasarguard")
			if tc.fails {
				if err == nil {
					t.Fatalf("expected an error, got %s %s", driver, dsn)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if ok != tc.ok || driver != tc.driver || dsn != tc.dsn {
				t.Fatalf("got (%q, %q, %v), want (%q, %q, %v)", driver, dsn, ok, tc.driver, tc.dsn, tc.ok)
			}
		})
	}
}

func TestPasarGuardEnvValue(t *testing.T) {
	env := strings.Join([]string{
		`# SQLALCHEMY_DATABASE_URL = "sqlite+aiosqlite:///db.sqlite3"`,
		`SQLALCHEMY_DATABASE_URL = "sqlite+aiosqlite:////var/lib/pasarguard/db.sqlite3"`,
		`#SQLALCHEMY_DATABASE_URL = "sqlite`,
		``,
		`# SQLAlchemy Database URL`,
		`SQLALCHEMY_DATABASE_URL="postgresql+asyncpg://pasarguard:secret@127.0.0.1:6432/pasarguard"`,
		`UVICORN_PORT = 8000`,
	}, "\n")
	value, found := pasarGuardEnvValue(env, "SQLALCHEMY_DATABASE_URL")
	if !found || value != "postgresql+asyncpg://pasarguard:secret@127.0.0.1:6432/pasarguard" {
		t.Fatalf("got %q, %v", value, found)
	}
	if _, found := pasarGuardEnvValue(env, "DATABASE_URL"); found {
		t.Fatal("DATABASE_URL must not match SQLALCHEMY_DATABASE_URL")
	}
}

func TestDetectPasarGuardSourcePrefersEnvOverLeftoverSQLite(t *testing.T) {
	dir := t.TempDir()
	leftover := filepath.Join(dir, "db.sqlite3")
	if err := os.WriteFile(leftover, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	envFile := filepath.Join(dir, ".env")
	env := "SQLALCHEMY_DATABASE_URL = \"postgresql+asyncpg://pasarguard:secret@127.0.0.1:6432/pasarguard\"\n"
	if err := os.WriteFile(envFile, []byte(env), 0o600); err != nil {
		t.Fatal(err)
	}

	driver, dsn, err := detectPasarGuardSourceIn([]string{envFile}, []string{leftover})
	if err != nil {
		t.Fatal(err)
	}
	if driver != "postgres" || !strings.HasPrefix(dsn, "postgresql://pasarguard:secret@127.0.0.1:6432/pasarguard?") {
		t.Fatalf("got %s %s", driver, dsn)
	}
}

func TestDetectPasarGuardSourceFallsBackToSQLiteFile(t *testing.T) {
	dir := t.TempDir()
	dbFile := filepath.Join(dir, "db.sqlite3")
	if err := os.WriteFile(dbFile, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	// A relative path resolves inside the PasarGuard container, not here.
	envFile := filepath.Join(dir, "missing", ".env")
	if err := os.MkdirAll(filepath.Dir(envFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(envFile, []byte("SQLALCHEMY_DATABASE_URL = \"sqlite+aiosqlite:///db.sqlite3\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	driver, dsn, err := detectPasarGuardSourceIn([]string{envFile}, []string{dbFile})
	if err != nil {
		t.Fatal(err)
	}
	if driver != "sqlite" || dsn != dbFile {
		t.Fatalf("got %s %s", driver, dsn)
	}
}

// "file:"+"//var/..." made SQLite read "var" as a URI authority and fail
// with "out of memory"; ? and # in a path cut the URI short.
func TestSQLiteReadOnlyURIOpensUnusualPaths(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "pasar guard?#%")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "db.sqlite3")
	db, err := gorm.Open(sqlite.Open((&url.URL{Scheme: "file", Path: path}).String()), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("CREATE TABLE users (id INTEGER)").Error; err != nil {
		t.Fatal(err)
	}
	closeGormDB(db)

	for _, p := range []string{path, "/" + path} {
		ro, err := gorm.Open(sqlite.Open(sqliteReadOnlyURI(p)), &gorm.Config{})
		if err != nil {
			t.Fatalf("open %q: %v", p, err)
		}
		var n int64
		if err := ro.Table("users").Count(&n).Error; err != nil {
			t.Fatalf("query %q: %v", p, err)
		}
		if err := ro.Exec("INSERT INTO users (id) VALUES (1)").Error; err == nil {
			t.Fatalf("%q opened writable", p)
		}
		closeGormDB(ro)
	}
}
