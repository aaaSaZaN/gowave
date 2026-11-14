package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gowave/internal/database"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func TestPostgresSelectExpressionAndPlaceholder(t *testing.T) {
	tests := []struct {
		column          postgresColumn
		wantSelect      string
		wantPlaceholder string
	}{
		{postgresColumn{Name: "uuid", DataType: "uuid", UdtName: "uuid"}, `CAST("uuid" AS text) AS "uuid"`, "CAST(? AS uuid)"},
		{postgresColumn{Name: "config", DataType: "jsonb", UdtName: "jsonb"}, `CAST("config" AS text) AS "config"`, "CAST(? AS jsonb)"},
		{postgresColumn{Name: "status", DataType: "USER-DEFINED", UdtName: "USERS_STATUS"}, `CAST("status" AS text) AS "status"`, `CAST(? AS "USERS_STATUS")`},
		{postgresColumn{Name: "tags", DataType: "ARRAY", UdtName: "_text"}, `CAST("tags" AS text) AS "tags"`, `CAST(? AS "text"[])`},
		{postgresColumn{Name: "integration_uuids", DataType: "ARRAY", UdtName: "_uuid"}, `CAST("integration_uuids" AS text) AS "integration_uuids"`, `CAST(? AS "uuid"[])`},
		{postgresColumn{Name: "amount", DataType: "numeric", UdtName: "numeric"}, `CAST("amount" AS text) AS "amount"`, "CAST(? AS numeric)"},
		{postgresColumn{Name: "username", DataType: "character varying", UdtName: "varchar"}, `"username"`, "?"},
		{postgresColumn{Name: "created_at", DataType: "timestamp with time zone", UdtName: "timestamptz"}, `"created_at"`, "?"},
		{postgresColumn{Name: "is_disabled", DataType: "boolean", UdtName: "bool"}, `"is_disabled"`, "?"},
		{postgresColumn{Name: "public_key", DataType: "bytea", UdtName: "bytea"}, `"public_key"`, "?"},
	}
	for _, tt := range tests {
		if got := postgresSelectExpression(tt.column); got != tt.wantSelect {
			t.Errorf("postgresSelectExpression(%s) = %s, want %s", tt.column.Name, got, tt.wantSelect)
		}
		if got := postgresPlaceholder(tt.column); got != tt.wantPlaceholder {
			t.Errorf("postgresPlaceholder(%s) = %s, want %s", tt.column.Name, got, tt.wantPlaceholder)
		}
	}
}

func TestAssertUpstreamSource(t *testing.T) {
	var complete []string
	for table := range requiredUpstreamTables {
		complete = append(complete, table)
	}
	if err := assertUpstreamSource(complete); err != nil {
		t.Errorf("complete upstream table list rejected: %v", err)
	}

	err := assertUpstreamSource(append(append([]string{}, complete...), migrationsTable))
	if !errors.Is(err, ErrSourceIsGoWave) {
		t.Errorf("GoWave source: got %v, want ErrSourceIsGoWave", err)
	}

	err = assertUpstreamSource([]string{"users", "nodes"})
	if err == nil || !strings.Contains(err.Error(), "missing tables") {
		t.Errorf("incomplete source: got %v, want a missing-tables error", err)
	}
}

func TestSameDatabase(t *testing.T) {
	address, other := "10.0.0.1", "10.0.0.2"
	port, otherPort := 5432, 6767
	base := databaseFingerprint{Database: "gowave", Schema: "public", Address: &address, Port: &port}

	if !sameDatabase(base, base) {
		t.Error("identical fingerprints reported as different databases")
	}
	socket := databaseFingerprint{Database: "gowave", Schema: "public"}
	if !sameDatabase(socket, socket) {
		t.Error("two Unix-socket connections to the same database reported as different")
	}
	if sameDatabase(base, socket) {
		t.Error("a TCP and a Unix-socket fingerprint must not be assumed identical")
	}

	differing := []databaseFingerprint{
		{Database: "upstream", Schema: "public", Address: &address, Port: &port},
		{Database: "gowave", Schema: "other", Address: &address, Port: &port},
		{Database: "gowave", Schema: "public", Address: &other, Port: &port},
		{Database: "gowave", Schema: "public", Address: &address, Port: &otherPort},
	}
	for _, candidate := range differing {
		if sameDatabase(base, candidate) {
			t.Errorf("%+v reported as the same database as %+v", candidate, base)
		}
	}
}

func TestIsMigrationCommandCoversPGToPG(t *testing.T) {
	for _, args := range [][]string{
		{"--migrate-pg-to-pg"},
		{"migrate-pg-to-pg"},
		{"migrate", "pg-to-pg"},
		{"--migrate", "postgres-to-postgres"},
	} {
		if !IsMigrationCommand(args) {
			t.Errorf("IsMigrationCommand(%v) = false, want true", args)
		}
	}
}

// TestMigratePostgresToPostgres_Integration migrates a fixture that carries the
// upstream panel's own PostgreSQL types — native enums, uuid, jsonb,
// timestamptz(3), numeric and arrays — plus upstream-only tables and columns,
// into the GoWave schema.
func TestMigratePostgresToPostgres_Integration(t *testing.T) {
	pgDSN := os.Getenv("GOWAVE_TEST_POSTGRES")
	if pgDSN == "" {
		t.Skip("Skipping integration test: GOWAVE_TEST_POSTGRES is not set")
	}

	source, sourceDSN := openIsolatedPostgres(t, pgDSN)
	applyUpstreamFixture(t, source)
	fixture := seedUpstreamSource(t, source)

	// The target mirrors `docker compose up`: GoWave already started on it and
	// seeded its defaults before the migration runs.
	target, targetDSN := openIsolatedPostgres(t, pgDSN)
	if err := database.AutoMigrate(target); err != nil {
		t.Fatalf("prepare target: %v", err)
	}

	report, err := MigratePostgresToPostgres(nil, sourceDSN, targetDSN, false)
	if err != nil {
		t.Fatalf("MigratePostgresToPostgres failed: %v", err)
	}
	assertUpstreamMigration(t, target, report, fixture)

	// Reruns are refused unless the caller asks for a replacement, and a forced
	// rerun must land exactly the same rows.
	_, err = MigratePostgresToPostgres(nil, sourceDSN, targetDSN, false)
	if !errors.Is(err, ErrPostgresTargetNotEmpty) {
		t.Fatalf("second run without force: got %v, want ErrPostgresTargetNotEmpty", err)
	}
	forced, err := MigratePostgresToPostgres(nil, sourceDSN, targetDSN, true)
	if err != nil {
		t.Fatalf("forced rerun failed: %v", err)
	}
	assertUpstreamMigration(t, target, forced, fixture)

	// The upstream source is only ever read.
	var sourceUsers int64
	if err := source.Table("users").Count(&sourceUsers).Error; err != nil {
		t.Fatal(err)
	}
	if sourceUsers != 2 {
		t.Errorf("source users = %d, want the 2 seeded rows left untouched", sourceUsers)
	}
	var prismaRows int64
	if err := source.Table("_prisma_migrations").Count(&prismaRows).Error; err != nil {
		t.Fatal(err)
	}
	if prismaRows != 1 {
		t.Errorf("source _prisma_migrations = %d, want 1", prismaRows)
	}
}

func TestMigratePostgresToPostgresRefusesSameDatabase(t *testing.T) {
	pgDSN := os.Getenv("GOWAVE_TEST_POSTGRES")
	if pgDSN == "" {
		t.Skip("Skipping integration test: GOWAVE_TEST_POSTGRES is not set")
	}

	source, sourceDSN := openIsolatedPostgres(t, pgDSN)
	applyUpstreamFixture(t, source)
	seedUpstreamSource(t, source)

	if _, err := MigratePostgresToPostgres(nil, sourceDSN, sourceDSN, true); !errors.Is(err, ErrSameDatabase) {
		t.Fatalf("source as its own target: got %v, want ErrSameDatabase", err)
	}
	// The refused run must not have created the GoWave schema in the source.
	if source.Migrator().HasTable(migrationsTable) {
		t.Errorf("the refused migration created %s in the upstream source", migrationsTable)
	}
}

func TestMigratePostgresToPostgresRefusesGoWaveSource(t *testing.T) {
	pgDSN := os.Getenv("GOWAVE_TEST_POSTGRES")
	if pgDSN == "" {
		t.Skip("Skipping integration test: GOWAVE_TEST_POSTGRES is not set")
	}

	source, sourceDSN := openIsolatedPostgres(t, pgDSN)
	if err := database.AutoMigrate(source); err != nil {
		t.Fatalf("prepare GoWave source: %v", err)
	}
	_, targetDSN := openIsolatedPostgres(t, pgDSN)

	if _, err := MigratePostgresToPostgres(nil, sourceDSN, targetDSN, true); !errors.Is(err, ErrSourceIsGoWave) {
		t.Fatalf("GoWave source: got %v, want ErrSourceIsGoWave", err)
	}
}

func TestMigratePostgresToPostgresRejectsIncompleteSource(t *testing.T) {
	pgDSN := os.Getenv("GOWAVE_TEST_POSTGRES")
	if pgDSN == "" {
		t.Skip("Skipping integration test: GOWAVE_TEST_POSTGRES is not set")
	}

	source, sourceDSN := openIsolatedPostgres(t, pgDSN)
	if err := source.Exec(`CREATE TABLE "users" ("id" bigint)`).Error; err != nil {
		t.Fatal(err)
	}
	_, targetDSN := openIsolatedPostgres(t, pgDSN)

	_, err := MigratePostgresToPostgres(nil, sourceDSN, targetDSN, true)
	if err == nil || !strings.Contains(err.Error(), "missing tables") {
		t.Fatalf("got %v, want a missing-tables error", err)
	}
}

// upstreamFixture records what the seeded source holds, so the assertions do
// not restate the literals twice.
type upstreamFixture struct {
	counts      map[string]int64
	userUUID    string
	nodeUUID    string
	squadUUID   string
	profileUUID string
	tags        []string
	ips         string
}

func applyUpstreamFixture(t *testing.T, db *gorm.DB) {
	t.Helper()
	path := filepath.Join("testdata", "upstream_postgres_schema.sql")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "--") {
			continue
		}
		if err := db.Exec(line).Error; err != nil {
			t.Fatalf("apply fixture statement %q: %v", line, err)
		}
	}
}

func seedUpstreamSource(t *testing.T, db *gorm.DB) upstreamFixture {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Millisecond)
	fixture := upstreamFixture{
		userUUID:    uuid.NewString(),
		nodeUUID:    uuid.NewString(),
		squadUUID:   uuid.NewString(),
		profileUUID: uuid.NewString(),
		// Characters a PostgreSQL array literal has to quote and escape.
		tags: []string{"pro, plus", `say "hi"`, `back\slash`, "{braced}"},
		ips:  `["10.0.0.1","10.0.0.2"]`,
	}
	inbound := uuid.NewString()
	template := uuid.NewString()
	subpage := uuid.NewString()
	provider := uuid.NewString()
	integration := uuid.NewString()
	arrayLiteral := "{" + strings.Join(quoteArrayElements(fixture.tags), ",") + "}"

	statements := []seedStatement{
		{`INSERT INTO admin (uuid, username, password_hash, role, created_at, updated_at, telegram_id) VALUES (?, 'root', '$argon2id$v=19$m=65536,t=3,p=4$abc$def', 'ADMIN', ?, ?, 777)`, []interface{}{uuid.NewString(), now, now}},
		{`INSERT INTO remnawave_settings (id, title, is_login_allowed, is_register_allowed, password_auth_enabled, passkey_settings, oauth2_settings, password_settings, branding_settings) VALUES (1, 'Upstream Panel', true, false, true, '{"enabled":true}', '{}', '{}', '{"logo":null}')`, nil},
		{`INSERT INTO api_tokens (uuid, name, scopes, created_at, updated_at) VALUES (?, 'full', '{users:read,"system:metadata"}', ?, ?)`, []interface{}{uuid.NewString(), now, now}},
		{`INSERT INTO api_tokens (uuid, name, scopes, created_at, updated_at) VALUES (?, 'none', '{}', ?, ?)`, []interface{}{uuid.NewString(), now, now}},
		{`INSERT INTO api_tokens (uuid, name, scopes, created_at, updated_at) VALUES (?, 'null', NULL, ?, ?)`, []interface{}{uuid.NewString(), now, now}},
		{`INSERT INTO subscription_settings (uuid, serve_json_at_base_subscription, is_show_custom_remarks, custom_remarks, custom_response_headers, randomize_hosts, response_rules, hwid_settings, created_at, updated_at) VALUES (?, true, true, '{"expired":["Expired"]}', '{}', false, '{"version":"1"}', '{"enabled":false}', ?, ?)`, []interface{}{uuid.NewString(), now, now}},
		{`INSERT INTO subscription_templates (uuid, name, template_type, template_yaml, template_json, created_at, updated_at) VALUES (?, 'Upstream Mihomo', 'MIHOMO', 'mixed-port: 7890', NULL, ?, ?)`, []interface{}{template, now, now}},
		{`INSERT INTO subscription_page_config (uuid, view_position, name, tags, config, created_at, updated_at) VALUES (?, 1, 'Upstream page', '{}', '{"theme":"dark"}', ?, ?)`, []interface{}{subpage, now, now}},
		{`INSERT INTO external_squads (uuid, name, tags, subscription_settings, host_overrides, response_headers_add, response_headers_remove, hwid_settings, custom_remarks, subpage_config_uuid, created_at, updated_at) VALUES (?, 'External', ?, '{}', '{}', '{}', '{x-trace}', '{}', '{}', ?, ?, ?)`, []interface{}{uuid.NewString(), arrayLiteral, subpage, now, now}},
		{`INSERT INTO config_profiles (uuid, name, tags, config, created_at, updated_at) VALUES (?, 'Upstream profile', ?, '{"inbounds":[]}', ?, ?)`, []interface{}{fixture.profileUUID, arrayLiteral, now, now}},
		{`INSERT INTO config_profile_inbounds (uuid, profile_uuid, tag, type, network, security, port, raw_inbound) VALUES (?, ?, 'VLESS_TCP', 'vless', 'tcp', 'reality', 443, '{"tag":"VLESS_TCP"}')`, []interface{}{inbound, fixture.profileUUID}},
		{`INSERT INTO infra_providers (uuid, name, favicon_link, login_url, created_at, updated_at) VALUES (?, 'Hoster', NULL, NULL, ?, ?)`, []interface{}{provider, now, now}},
		{`INSERT INTO integrations (uuid, name, description, config, created_at, updated_at) VALUES (?, 'hoster-api', NULL, '{"token":"x"}', ?, ?)`, []interface{}{integration, now, now}},
		{`INSERT INTO nodes (id, uuid, name, address, port, country_code, tags, integration_uuids, ips, active_config_profile_uuid, provider_uuid, note, created_at, updated_at, xray_version, xray_uptime) VALUES (7, ?, 'Upstream node', '10.0.0.1', 443, 'NL', ?, ?, ?, ?, ?, NULL, ?, ?, '25.1.1', '3d')`, []interface{}{fixture.nodeUUID, arrayLiteral, "{" + integration + "}", fixture.ips, fixture.profileUUID, provider, now, now}},
		{`INSERT INTO infra_billing_nodes (uuid, node_uuid, name, provider_uuid, next_billing_at, created_at, updated_at) VALUES (?, ?, 'Billing', ?, ?, ?, ?)`, []interface{}{uuid.NewString(), fixture.nodeUUID, provider, now, now, now}},
		{`INSERT INTO infra_billing_history (uuid, provider_uuid, amount, billed_at) VALUES (?, ?, 12.34, ?)`, []interface{}{uuid.NewString(), provider, now}},
		{`INSERT INTO hosts (uuid, remark, address, port, security_layer, xhttp_extra_params, mux_params, sockopt_params, tags, exclude_from_subscription_types, mapper, config_profile_uuid, config_profile_inbound_uuid, xray_json_template_uuid, allow_insecure) VALUES (?, 'Host', 'example.com', 443, 'TLS', NULL, '{"enabled":true}', NULL, ?, '{STASH}', '{}', ?, ?, ?, true)`, []interface{}{uuid.NewString(), arrayLiteral, fixture.profileUUID, inbound, template}},
		{`INSERT INTO internal_squads (uuid, name, tags, created_at, updated_at) VALUES (?, 'Internal', '{}', ?, ?)`, []interface{}{fixture.squadUUID, now, now}},
		{`INSERT INTO internal_squad_inbounds (internal_squad_uuid, inbound_uuid) VALUES (?, ?)`, []interface{}{fixture.squadUUID, inbound}},
		{`INSERT INTO config_profile_inbounds_to_nodes (config_profile_inbound_uuid, node_uuid) VALUES (?, ?)`, []interface{}{inbound, fixture.nodeUUID}},
		{`INSERT INTO entity_meta (entity_id, entity_type, metadata) VALUES ('7', 'NODE', '{"rack":"a1"}')`, nil},
		{`INSERT INTO config_profile_snippets (name, snippet, created_at) VALUES ('block-ads', '{"rules":[]}', ?)`, []interface{}{now}},
		{`INSERT INTO shared_lists (name, config, created_at, updated_at) VALUES ('geosite', '{"items":[]}', ?, ?)`, []interface{}{now, now}},
		{`INSERT INTO _prisma_migrations (id, checksum, migration_name, finished_at, applied_steps_count) VALUES ('1', 'abc', '20240101_init', ?, 1)`, []interface{}{now}},
		// More rows than one INSERT batch holds.
		{`INSERT INTO nodes_usage_history (node_uuid, created_at, download_bytes, upload_bytes, total_bytes, updated_at) SELECT ?, timestamptz '2026-01-01 00:00:00Z' + (n * interval '1 minute'), n, n, 2 * n, timestamptz '2026-01-01 00:00:00Z' FROM generate_series(1, 1203) AS n`, []interface{}{fixture.nodeUUID}},
	}

	for userID := 1; userID <= 2; userID++ {
		name := fmt.Sprintf("upstream-user-%d", userID)
		status, strategy := "ACTIVE", "NO_RESET"
		if userID == 2 {
			status, strategy = "LIMITED", "MONTH"
		}
		userUUID := uuid.NewString()
		vless := uuid.NewString()
		if userID == 1 {
			fixture.userUUID = vless
		}
		statements = append(statements,
			// The second user leaves every optional column NULL, including the
			// ones GoWave stores in a non-pointer Go field.
			seedStatement{`INSERT INTO users (revision, id, short_uuid, username, status, traffic_limit_bytes, traffic_limit_strategy, expire_at, trojan_password, vless_uuid, ss_password, description, tag, telegram_id, email, hwid_device_limit, last_triggered_threshold, created_at, updated_at, uuid, first_connected_at) VALUES (1, ?, ?, ?, CAST(? AS "USERS_STATUS"), 1024, CAST(? AS "RESET_PERIODS"), ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, ?, ?, ?, ?)`, []interface{}{
				userID, name, name, status, strategy, now.Add(24 * time.Hour),
				nullableFor(userID, "trojan-"+name), vless, nullableFor(userID, "ss-"+name),
				nullableFor(userID, "description "+name), nullableFor(userID, "vip"),
				nullableFor(userID, int64(100+userID)), nullableFor(userID, name+"@example.com"),
				nullableFor(userID, 3), now, now, userUUID, now,
			}},
			seedStatement{`INSERT INTO user_traffic (id, used_traffic_bytes, lifetime_used_traffic_bytes, online_at, last_connected_node_uuid, first_connected_at) VALUES (?, 2048, 4096, ?, ?, ?)`, []interface{}{userID, now, fixture.nodeUUID, now}},
			seedStatement{`INSERT INTO internal_squad_members (internal_squad_uuid, user_id) VALUES (?, ?)`, []interface{}{fixture.squadUUID, userID}},
			seedStatement{`INSERT INTO hwid_user_devices (hwid, user_id, platform, os_version, device_model, user_agent, request_ip, created_at, updated_at) VALUES (?, ?, 'android', '14', 'Pixel', 'okhttp', '10.1.1.1', ?, ?)`, []interface{}{name + "-hwid", userID, now, now}},
			seedStatement{`INSERT INTO nodes_user_usage_history (node_id, created_at, user_id, total_bytes, updated_at) VALUES (7, ?, ?, 8192, ?)`, []interface{}{now, userID, now}},
			seedStatement{`INSERT INTO user_subscription_request_history (id, user_id, request_ip, user_agent, srr_rule_name, srr_response_type, request_at) VALUES (?, ?, '10.1.1.1', 'v2rayNG', 'Fallback Base64', 'XRAY_BASE64', ?)`, []interface{}{userID, userID, now}},
		)
	}

	for _, statement := range statements {
		if err := db.Exec(statement.sql, statement.args...).Error; err != nil {
			t.Fatalf("seed upstream source: %s: %v", statement.sql, err)
		}
	}

	tables, err := listPostgresCurrentTables(db)
	if err != nil {
		t.Fatal(err)
	}
	fixture.counts = tableCounts(t, db, tables)
	return fixture
}

// nullableFor gives the first user a value and the second one SQL NULL.
func nullableFor(userID int, value interface{}) interface{} {
	if userID == 1 {
		return value
	}
	return nil
}

// quoteArrayElements renders array elements the way PostgreSQL's array literal
// parser expects, so the fixture can seed values that need escaping.
func quoteArrayElements(values []string) []string {
	quoted := make([]string, len(values))
	for i, value := range values {
		escaped := strings.ReplaceAll(value, `\`, `\\`)
		escaped = strings.ReplaceAll(escaped, `"`, `\"`)
		quoted[i] = `"` + escaped + `"`
	}
	return quoted
}

func assertUpstreamMigration(t *testing.T, target *gorm.DB, report *PGToPGReport, fixture upstreamFixture) {
	t.Helper()

	for table, want := range fixture.counts {
		if table == "_prisma_migrations" {
			continue
		}
		if report.Rows[table] != want {
			t.Errorf("%s: report says %d rows, source has %d", table, report.Rows[table], want)
		}
		// GoWave seeds one template per type it ships, so this table legitimately
		// ends up with more rows than the source had; it is checked separately.
		if table == "subscription_templates" {
			continue
		}
		var got int64
		if err := target.Table(table).Count(&got).Error; err != nil {
			t.Errorf("count %s in the target: %v", table, err)
			continue
		}
		if got != want {
			t.Errorf("%s: target has %d rows, source has %d", table, got, want)
		}
	}

	if !containsString(report.SkippedTables, "_prisma_migrations") {
		t.Errorf("skipped tables = %v, want _prisma_migrations among them", report.SkippedTables)
	}
	for _, column := range []string{"users.uuid", "users.first_connected_at", "nodes.xray_version", "hosts.allow_insecure", "admin.telegram_id"} {
		if !containsString(report.DroppedColumns, column) {
			t.Errorf("dropped columns = %v, want %s among them", report.DroppedColumns, column)
		}
	}
	// GoWave-only tables must be reported, and the outbox must stay empty so no
	// stale work is replayed against the nodes.
	for _, table := range []string{"jobs", "keygen", "user_meta", "node_meta"} {
		if !containsString(report.EmptyTargetTables, table) {
			t.Errorf("empty target tables = %v, want %s among them", report.EmptyTargetTables, table)
		}
		var count int64
		if err := target.Table(table).Count(&count).Error; err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if count != 0 {
			t.Errorf("%s has %d rows, want it empty", table, count)
		}
	}

	// Enums, uuid, non-pointer NULLs and jsonb all land as GoWave text.
	var first database.User
	if err := target.Where("username = ?", "upstream-user-1").First(&first).Error; err != nil {
		t.Fatalf("migrated user 1: %v", err)
	}
	if first.Status != "ACTIVE" || first.TrafficLimitStrategy != "NO_RESET" {
		t.Errorf("user 1 status/strategy = %q/%q, want ACTIVE/NO_RESET", first.Status, first.TrafficLimitStrategy)
	}
	if first.VlessUUID != fixture.userUUID {
		t.Errorf("user 1 vless_uuid = %q, want %q", first.VlessUUID, fixture.userUUID)
	}
	if first.Description != "description upstream-user-1" || first.Email != "upstream-user-1@example.com" {
		t.Errorf("user 1 description/email = %q/%q", first.Description, first.Email)
	}
	if first.TelegramID == nil || *first.TelegramID != 101 {
		t.Errorf("user 1 telegram_id = %v, want 101", first.TelegramID)
	}
	if first.HWIDDeviceLimit == nil || *first.HWIDDeviceLimit != 3 {
		t.Errorf("user 1 hwid_device_limit = %v, want 3", first.HWIDDeviceLimit)
	}

	var second database.User
	if err := target.Where("username = ?", "upstream-user-2").First(&second).Error; err != nil {
		t.Fatalf("migrated user 2: %v", err)
	}
	if second.Status != "LIMITED" || second.TrafficLimitStrategy != "MONTH" {
		t.Errorf("user 2 status/strategy = %q/%q, want LIMITED/MONTH", second.Status, second.TrafficLimitStrategy)
	}
	// A NULL in a column GoWave stores in a non-pointer field becomes its zero value.
	if second.Description != "" || second.Email != "" || second.TrojanPassword != "" {
		t.Errorf("user 2 kept %q/%q/%q, want empty strings", second.Description, second.Email, second.TrojanPassword)
	}
	if second.TelegramID != nil || second.HWIDDeviceLimit != nil {
		t.Errorf("user 2 pointer fields = %v/%v, want NULL", second.TelegramID, second.HWIDDeviceLimit)
	}
	if report.NormalizedNulls == 0 {
		t.Error("report.NormalizedNulls = 0, want the NULLs mapped to Go zero values counted")
	}

	// Arrays survive quoting, escaping and braces; uuid[] stays uuid[].
	var node database.Node
	if err := target.Where("uuid = ?", fixture.nodeUUID).First(&node).Error; err != nil {
		t.Fatalf("migrated node: %v", err)
	}
	if got := strings.Join(node.Tags, "|"); got != strings.Join(fixture.tags, "|") {
		t.Errorf("node tags = %q, want %q", got, strings.Join(fixture.tags, "|"))
	}
	// Upstream keeps ips as jsonb, whose text form is canonical rather than
	// byte-identical to what was inserted, so compare the parsed documents.
	var gotIPs, wantIPs []string
	if err := json.Unmarshal([]byte(node.IPs), &gotIPs); err != nil {
		t.Fatalf("node ips %q is not JSON: %v", node.IPs, err)
	}
	if err := json.Unmarshal([]byte(fixture.ips), &wantIPs); err != nil {
		t.Fatal(err)
	}
	if strings.Join(gotIPs, "|") != strings.Join(wantIPs, "|") {
		t.Errorf("node ips = %v, want %v", gotIPs, wantIPs)
	}
	if len(node.IntegrationUUIDs) != 1 {
		t.Errorf("node integration_uuids = %v, want one element", node.IntegrationUUIDs)
	}
	if node.ID != 7 {
		t.Errorf("node id = %d, want the upstream 7", node.ID)
	}

	var profile database.ConfigProfile
	if err := target.Where("uuid = ?", fixture.profileUUID).First(&profile).Error; err != nil {
		t.Fatalf("migrated config profile: %v", err)
	}
	if !strings.Contains(profile.Config, `"inbounds"`) {
		t.Errorf("config profile config = %q, want the upstream jsonb document", profile.Config)
	}

	// numeric values keep their scale through the text round-trip.
	var amount string
	if err := target.Raw(`SELECT CAST(amount AS text) FROM infra_billing_history LIMIT 1`).Scan(&amount).Error; err != nil {
		t.Fatal(err)
	}
	if amount != "12.34" {
		t.Errorf("billing amount = %q, want 12.34", amount)
	}

	// Empty and NULL text[] values both read back as an empty scope list.
	var tokens []database.ApiToken
	if err := target.Order("name").Find(&tokens).Error; err != nil {
		t.Fatal(err)
	}
	if len(tokens) != 3 {
		t.Fatalf("api tokens = %d, want 3", len(tokens))
	}
	wantScopes := map[string][]string{
		"full": {"users:read", "system:metadata"},
		"none": {},
		"null": {},
	}
	for _, token := range tokens {
		want := wantScopes[token.Name]
		if strings.Join(token.Scopes, ",") != strings.Join(want, ",") {
			t.Errorf("token %s scopes = %v, want %v", token.Name, []string(token.Scopes), want)
		}
	}

	// GoWave seeds the templates and the page configuration the upstream panel
	// did not supply, without duplicating the ones it did.
	var mihomo int64
	if err := target.Table("subscription_templates").Where("template_type = ?", "MIHOMO").Count(&mihomo).Error; err != nil {
		t.Fatal(err)
	}
	if mihomo != 1 {
		t.Errorf("MIHOMO templates = %d, want the migrated one kept and not re-seeded", mihomo)
	}
	var singbox int64
	if err := target.Table("subscription_templates").Where("template_type = ?", "SINGBOX").Count(&singbox).Error; err != nil {
		t.Fatal(err)
	}
	if singbox != 1 {
		t.Errorf("SINGBOX templates = %d, want GoWave's default seeded", singbox)
	}

	// Sequences continue after the copied IDs instead of colliding with them.
	next := database.User{Username: "after-migration", ShortUUID: "after-migration", VlessUUID: uuid.NewString()}
	if err := target.Create(&next).Error; err != nil {
		t.Fatalf("insert after migration: %v", err)
	}
	if next.ID <= 2 {
		t.Errorf("new user id = %d, want it after the migrated ids", next.ID)
	}
	if err := target.Delete(&next).Error; err != nil {
		t.Fatalf("clean up the probe user: %v", err)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
