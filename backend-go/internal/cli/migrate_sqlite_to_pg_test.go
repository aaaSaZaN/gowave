package cli

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gowave/config"
	"gowave/internal/database"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func TestIsMigrationCommand(t *testing.T) {
	tests := []struct {
		args     []string
		expected bool
	}{
		{[]string{"--migrate-pg-to-sqlite"}, true},
		{[]string{"migrate-pg-to-sqlite"}, true},
		{[]string{"--migrate-sqlite-to-pg"}, true},
		{[]string{"migrate-sqlite-to-pg"}, true},
		{[]string{"migrate", "sqlite-to-pg"}, true},
		{[]string{"migrate", "to-docker", "--yes"}, true},
		{[]string{"--migrate-to-docker"}, true},
		{[]string{"--migrate", "pg-to-sqlite"}, true},
		{[]string{"--rescue"}, false},
		{[]string{"import", "pasarguard"}, false},
		{[]string{}, false},
	}

	for _, tt := range tests {
		got := IsMigrationCommand(tt.args)
		if got != tt.expected {
			t.Errorf("IsMigrationCommand(%v) = %v; want %v", tt.args, got, tt.expected)
		}
	}
}

func TestDetectMigrationPaths(t *testing.T) {
	tmpDir := t.TempDir()
	sqliteFile := filepath.Join(tmpDir, "test.sqlite")
	if err := os.WriteFile(sqliteFile, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		DBDriver:    "sqlite",
		DatabaseURL: sqliteFile,
	}

	detected, err := DetectGoWaveSQLitePath(cfg)
	if err != nil {
		t.Fatalf("DetectGoWaveSQLitePath failed: %v", err)
	}
	if detected != sqliteFile {
		t.Errorf("got %s, want %s", detected, sqliteFile)
	}

	cfgPG := &config.Config{
		DBDriver:    "postgres",
		DatabaseURL: "postgresql://custom:custom@localhost:5432/custom_db",
	}
	dsn, err := DetectPostgresTargetDSN(cfgPG)
	if err != nil || dsn != cfgPG.DatabaseURL {
		t.Errorf("got %s (%v), want %s", dsn, err, cfgPG.DatabaseURL)
	}
}

func TestDetectPostgresTargetDSNRequiresPassword(t *testing.T) {
	for _, key := range []string{"DATABASE_URL", "POSTGRES_DSN", "POSTGRES_PASSWORD", "POSTGRES_USER", "POSTGRES_DB", "POSTGRES_PORT"} {
		t.Setenv(key, "")
	}
	if dsn, err := DetectPostgresTargetDSN(&config.Config{}); err == nil {
		t.Fatalf("expected an error without POSTGRES_PASSWORD, got DSN %q", dsn)
	}

	t.Setenv("POSTGRES_PASSWORD", "p@ss word")
	dsn, err := DetectPostgresTargetDSN(&config.Config{})
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("DSN %q does not parse: %v", dsn, err)
	}
	if pass, _ := parsed.User.Password(); pass != "p@ss word" {
		t.Errorf("password = %q, want it escaped and preserved", pass)
	}
}

func TestRedactDSN(t *testing.T) {
	tests := map[string]string{
		"postgresql://gowave:secret@127.0.0.1:5432/gowave?sslmode=disable": "postgresql://gowave:xxxxx@127.0.0.1:5432/gowave?sslmode=disable",
		"host=db user=gowave password=secret dbname=gowave":                "host=db user=gowave password=xxxxx dbname=gowave",
		"host=db password='se cret' dbname=gowave":                         "host=db password=xxxxx dbname=gowave",
	}
	for dsn, want := range tests {
		if got := redactDSN(dsn); got != want {
			t.Errorf("redactDSN(%q) = %q, want %q", dsn, got, want)
		}
	}
}

// TestMigrateSQLiteToPostgres_Integration fills every table that references
// users, nodes, hosts, templates or providers, because clearing tables one by
// one with CASCADE used to wipe already copied children.
func TestMigrateSQLiteToPostgres_Integration(t *testing.T) {
	pgDSN := os.Getenv("GOWAVE_TEST_POSTGRES")
	if pgDSN == "" {
		t.Skip("Skipping integration test: GOWAVE_TEST_POSTGRES is not set")
	}
	time.Local = time.UTC

	sqlitePath := filepath.Join(t.TempDir(), "source.sqlite")
	source, err := database.Init("sqlite", sqlitePath)
	if err != nil {
		t.Fatalf("open test sqlite: %v", err)
	}
	if err := database.AutoMigrate(source); err != nil {
		t.Fatalf("migrate test sqlite schema: %v", err)
	}
	seedMigrationSource(t, source)
	sourceTables, err := listSQLiteTables(source)
	if err != nil {
		t.Fatal(err)
	}
	sourceCounts := tableCounts(t, source, sourceTables)
	_ = closeGormDB(source)

	// The target mirrors `docker compose up`: GoWave already started on it
	// and seeded its defaults before the migration runs.
	target, targetDSN := openIsolatedPostgres(t, pgDSN)
	if err := database.AutoMigrate(target); err != nil {
		t.Fatalf("seed target: %v", err)
	}

	if err := MigrateSQLiteToPostgres(nil, sqlitePath, targetDSN, false); err != nil {
		t.Fatalf("MigrateSQLiteToPostgres failed: %v", err)
	}
	assertCounts(t, target, sourceCounts)

	err = MigrateSQLiteToPostgres(nil, sqlitePath, targetDSN, false)
	if !errors.Is(err, ErrPostgresTargetNotEmpty) {
		t.Fatalf("second run without force: got %v, want ErrPostgresTargetNotEmpty", err)
	}
	assertCounts(t, target, sourceCounts)

	if err := MigrateSQLiteToPostgres(nil, sqlitePath, targetDSN, true); err != nil {
		t.Fatalf("forced re-run failed: %v", err)
	}
	assertCounts(t, target, sourceCounts)

	var migrated database.User
	if err := target.Where("username = ?", "migration-user-1").First(&migrated).Error; err != nil {
		t.Fatalf("migrated user not found in postgres: %v", err)
	}
	if migrated.ExternalSquadUUID == nil || *migrated.ExternalSquadUUID != "es-1" {
		t.Errorf("external squad link lost: %v", migrated.ExternalSquadUUID)
	}

	// Sequences continue after the copied IDs.
	next := database.User{Username: "after-migration", ShortUUID: "after-migration", VlessUUID: uuid.NewString()}
	if err := target.Create(&next).Error; err != nil {
		t.Fatalf("insert after migration: %v", err)
	}
	if next.ID <= 2 {
		t.Errorf("new user id = %d, want it after the migrated ids", next.ID)
	}
}

type seedStatement struct {
	sql  string
	args []interface{}
}

func seedMigrationSource(t *testing.T, db *gorm.DB) {
	t.Helper()
	now := time.Now().UTC()
	var subpage string
	if err := db.Raw("SELECT uuid FROM subscription_page_config LIMIT 1").Scan(&subpage).Error; err != nil || subpage == "" {
		t.Fatalf("seeded subscription page config missing: %v", err)
	}
	statements := []seedStatement{
		{"INSERT INTO subscription_templates (uuid, name, template_type, template_json, created_at, updated_at) VALUES ('tpl-1', 'Custom', 'XRAY_JSON', '{}', ?, ?)", []interface{}{now, now}},
		{"INSERT INTO config_profiles (uuid, name, config, created_at, updated_at) VALUES ('cp-1', 'Profile', '{}', ?, ?)", []interface{}{now, now}},
		{"INSERT INTO config_profile_inbounds (uuid, profile_uuid, tag, type, port, raw_inbound) VALUES ('in-1', 'cp-1', 'VLESS', 'vless', 443, '{}')", nil},
		{"INSERT INTO infra_providers (uuid, name, created_at, updated_at) VALUES ('prov-1', 'Provider', ?, ?)", []interface{}{now, now}},
		{"INSERT INTO nodes (uuid, name, address, active_config_profile_uuid, provider_uuid, created_at, updated_at) VALUES ('node-1', 'Node 1', '10.0.0.1', 'cp-1', 'prov-1', ?, ?)", []interface{}{now, now}},
		{"INSERT INTO external_squads (uuid, name, subpage_config_uuid, created_at, updated_at) VALUES ('es-1', 'External', ?, ?, ?)", []interface{}{subpage, now, now}},
		{"INSERT INTO external_squads_templates (external_squad_uuid, template_type, template_uuid) VALUES ('es-1', 'XRAY_JSON', 'tpl-1')", nil},
		{"INSERT INTO internal_squads (uuid, name, created_at, updated_at) VALUES ('is-1', 'Internal', ?, ?)", []interface{}{now, now}},
		{"INSERT INTO internal_squad_inbounds (internal_squad_uuid, inbound_uuid) VALUES ('is-1', 'in-1')", nil},
		{"INSERT INTO hosts (uuid, remark, address, port, config_profile_uuid, config_profile_inbound_uuid, xray_json_template_uuid) VALUES ('host-1', 'Host', 'example.com', 443, 'cp-1', 'in-1', 'tpl-1')", nil},
		{"INSERT INTO hosts_to_nodes (host_uuid, node_uuid) VALUES ('host-1', 'node-1')", nil},
		{"INSERT INTO internal_squad_host_links (squad_uuid, host_uuid) VALUES ('is-1', 'host-1')", nil},
		{"INSERT INTO config_profile_inbounds_to_nodes (config_profile_inbound_uuid, node_uuid) VALUES ('in-1', 'node-1')", nil},
		{"INSERT INTO infra_billing_nodes (uuid, provider_uuid, node_uuid, next_billing_at, created_at, updated_at) VALUES ('bill-node-1', 'prov-1', 'node-1', ?, ?, ?)", []interface{}{now, now, now}},
		{"INSERT INTO infra_billing_history (uuid, provider_uuid, amount, billed_at) VALUES ('bill-1', 'prov-1', 5, ?)", []interface{}{now}},
		// More rows than one INSERT batch holds.
		{"WITH RECURSIVE seq(n) AS (SELECT 1 UNION ALL SELECT n + 1 FROM seq WHERE n < 1203) INSERT INTO nodes_usage_history (node_uuid, created_at, download_bytes, upload_bytes, total_bytes, updated_at) SELECT 'node-1', datetime('2026-01-01', '+' || n || ' minutes'), n, n, 2 * n, datetime('2026-01-01') FROM seq", nil},
	}
	for userID := 1; userID <= 2; userID++ {
		name := fmt.Sprintf("migration-user-%d", userID)
		statements = append(statements,
			seedStatement{"INSERT INTO users (id, short_uuid, username, status, vless_uuid, external_squad_uuid, expire_at, created_at, updated_at) VALUES (?, ?, ?, 'ACTIVE', ?, 'es-1', ?, ?, ?)", []interface{}{userID, name, name, uuid.NewString(), now.Add(24 * time.Hour), now, now}},
			seedStatement{"INSERT INTO user_traffic (id, used_traffic_bytes, last_connected_node_uuid) VALUES (?, 1024, 'node-1')", []interface{}{userID}},
			seedStatement{"INSERT INTO hwid_user_devices (hwid, user_id, platform, created_at, updated_at) VALUES (?, ?, 'android', ?, ?)", []interface{}{name + "-hwid", userID, now, now}},
			seedStatement{"INSERT INTO internal_squad_members (internal_squad_uuid, user_id) VALUES ('is-1', ?)", []interface{}{userID}},
			seedStatement{"INSERT INTO nodes_user_usage_history (node_id, created_at, user_id, total_bytes, updated_at) VALUES (1, ?, ?, 2048, ?)", []interface{}{now, userID, now}},
		)
	}
	for _, statement := range statements {
		if err := db.Exec(statement.sql, statement.args...).Error; err != nil {
			t.Fatalf("seed source: %s: %v", statement.sql, err)
		}
	}
}

func openIsolatedPostgres(t *testing.T, dsn string) (*gorm.DB, string) {
	t.Helper()
	admin, err := database.Init("postgres", dsn)
	if err != nil {
		t.Fatalf("connect to postgres: %v", err)
	}
	schema := "t_migrate_" + strings.ReplaceAll(uuid.NewString()[:8], "-", "")
	if err := admin.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatalf("create test schema: %v", err)
	}
	t.Cleanup(func() {
		admin.Exec("DROP SCHEMA " + schema + " CASCADE")
		_ = closeGormDB(admin)
	})

	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	query := parsed.Query()
	query.Set("search_path", schema)
	parsed.RawQuery = query.Encode()
	isolated := parsed.String()

	db, err := database.Init("postgres", isolated)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = closeGormDB(db) })
	return db, isolated
}

func tableCounts(t *testing.T, db *gorm.DB, tables []string) map[string]int64 {
	t.Helper()
	counts := make(map[string]int64, len(tables))
	for _, table := range tables {
		if table == migrationsTable {
			continue
		}
		var count int64
		if err := db.Table(table).Count(&count).Error; err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		counts[table] = count
	}
	return counts
}

func assertCounts(t *testing.T, db *gorm.DB, want map[string]int64) {
	t.Helper()
	empty := 0
	for table, expected := range want {
		var got int64
		if err := db.Table(table).Count(&got).Error; err != nil {
			t.Errorf("count %s in postgres: %v", table, err)
			continue
		}
		if got != expected {
			t.Errorf("%s: postgres has %d rows, sqlite has %d", table, got, expected)
		}
		if expected == 0 {
			empty++
		}
	}
	if empty == len(want) {
		t.Fatal("source fixture is empty")
	}
}

func TestCheckSQLiteForeignKeysNamesOrphans(t *testing.T) {
	path := filepath.Join(t.TempDir(), "orphans.sqlite")
	db, err := database.Init("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer closeGormDB(db)
	if err := database.MigrateSchema(db); err != nil {
		t.Fatal(err)
	}
	if err := checkSQLiteForeignKeys(db); err != nil {
		t.Fatalf("clean database reported: %v", err)
	}
	if err := db.Exec("PRAGMA foreign_keys = OFF").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO user_traffic (id, used_traffic_bytes) VALUES (99999, 1)").Error; err != nil {
		t.Fatal(err)
	}
	err = checkSQLiteForeignKeys(db)
	if err == nil || !strings.Contains(err.Error(), "user_traffic rowid 99999 references a missing users row") {
		t.Fatalf("got %v", err)
	}
}
