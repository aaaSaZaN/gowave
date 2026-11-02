package cli

import (
	"bufio"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"gowave/config"
	"gowave/internal/database"
)

// ErrSameDatabase is returned when the upstream source and the GoWave target
// resolve to the same PostgreSQL database and schema.
var ErrSameDatabase = errors.New("source and target point at the same PostgreSQL database")

// ErrSourceIsGoWave is returned when the source already carries the GoWave
// schema, so it is not an upstream panel database.
var ErrSourceIsGoWave = errors.New("source database already holds the GoWave schema")

// pgToPGBatchRows bounds a multi-row INSERT; the column count further limits it
// so a batch stays under PostgreSQL's 65535 bind parameters.
const pgToPGBatchRows = 500

// pgToPGMaxBindParams is PostgreSQL's hard limit on bind parameters per statement.
const pgToPGMaxBindParams = 65535

// databaseFingerprint identifies a connection's database well enough to refuse a
// migration onto itself.
type databaseFingerprint struct {
	Database string  `gorm:"column:database"`
	Schema   string  `gorm:"column:schema"`
	Address  *string `gorm:"column:address"`
	Port     *int    `gorm:"column:port"`
}

// PGToPGReport summarises one PostgreSQL-to-PostgreSQL migration so callers can
// print it or assert on it in tests.
type PGToPGReport struct {
	// Rows holds the number of copied rows per target table.
	Rows map[string]int64
	// SkippedTables lists source tables with no counterpart in GoWave.
	SkippedTables []string
	// DroppedColumns lists "table.column" pairs the GoWave schema does not have.
	DroppedColumns []string
	// EmptyTargetTables lists GoWave tables the source could not fill.
	EmptyTargetTables []string
	// NormalizedNulls counts NULL values mapped to a Go zero value.
	NormalizedNulls int64
}

// MigratePostgresToPostgres copies an upstream panel PostgreSQL database into a
// GoWave PostgreSQL database. The source is only ever read, inside a read-only
// repeatable-read transaction; the target is written in a single transaction, so
// a failure leaves it exactly as it was. A target that already holds users,
// nodes or hosts is only replaced when force is set.
func MigratePostgresToPostgres(_ *config.Config, sourceDSN, targetDSN string, force bool) (*PGToPGReport, error) {
	if strings.TrimSpace(sourceDSN) == "" {
		detected, err := DetectUpstreamPostgresDSN()
		if err != nil {
			return nil, err
		}
		sourceDSN = detected
	}
	if strings.TrimSpace(targetDSN) == "" {
		return nil, errors.New("target PostgreSQL DSN cannot be empty")
	}

	fmt.Printf("[*] Connecting to the upstream panel PostgreSQL source (read-only): %s\n", redactDSN(sourceDSN))
	sourceDB, err := gorm.Open(postgres.Open(sourceDSN), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return nil, fmt.Errorf("connect to the source PostgreSQL: %w", err)
	}
	defer closeGormDB(sourceDB)

	fmt.Printf("[*] Connecting to the GoWave PostgreSQL target: %s\n", redactDSN(targetDSN))
	targetDB, err := gorm.Open(postgres.Open(targetDSN), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return nil, fmt.Errorf("connect to the target PostgreSQL: %w", err)
	}
	defer closeGormDB(targetDB)

	// Both guards run before the GoWave schema is applied: afterwards a source
	// that is really the target would look like a GoWave database either way.
	if err := assertDistinctDatabases(sourceDB, targetDB); err != nil {
		return nil, err
	}
	sourceTables, err := listPostgresCurrentTables(sourceDB)
	if err != nil {
		return nil, fmt.Errorf("list source tables: %w", err)
	}
	if err := assertUpstreamSource(sourceTables); err != nil {
		return nil, err
	}

	fmt.Println("[*] Applying the GoWave schema to the target...")
	if err := database.MigrateSchema(targetDB); err != nil {
		return nil, fmt.Errorf("initialize the target schema: %w", err)
	}

	if !force {
		existing, err := postgresTargetData(targetDB)
		if err != nil {
			return nil, fmt.Errorf("inspect the target database: %w", err)
		}
		if len(existing) > 0 {
			return nil, fmt.Errorf("%w (%s); rerun with --force to replace it", ErrPostgresTargetNotEmpty, strings.Join(existing, ", "))
		}
	}

	report := &PGToPGReport{Rows: map[string]int64{}}
	err = sourceDB.Transaction(func(source *gorm.DB) error {
		return targetDB.Transaction(func(target *gorm.DB) error {
			return copyPostgresDatabase(source, target, sourceTables, report)
		})
	}, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return nil, fmt.Errorf("copy upstream data: %w", err)
	}

	// GoWave owns tables and rows the upstream panel never had, such as the
	// default subscription page configuration and the built-in templates.
	fmt.Println("[*] Seeding GoWave defaults the upstream panel does not provide...")
	database.SeedDefaults(targetDB)

	printPGToPGReport(report)
	return report, nil
}

// assertDistinctDatabases refuses a migration whose source and target are the
// same database and schema, which would truncate the upstream panel's data.
func assertDistinctDatabases(source, target *gorm.DB) error {
	sourcePrint, err := readDatabaseFingerprint(source)
	if err != nil {
		return fmt.Errorf("identify the source database: %w", err)
	}
	targetPrint, err := readDatabaseFingerprint(target)
	if err != nil {
		return fmt.Errorf("identify the target database: %w", err)
	}
	if sourcePrint != nil && targetPrint != nil && sameDatabase(*sourcePrint, *targetPrint) {
		return fmt.Errorf("%w (%s.%s); pass the upstream panel DSN as the source and the GoWave DSN as the target",
			ErrSameDatabase, sourcePrint.Database, sourcePrint.Schema)
	}
	return nil
}

func readDatabaseFingerprint(db *gorm.DB) (*databaseFingerprint, error) {
	var print databaseFingerprint
	err := db.Raw(`SELECT current_database() AS database,
		current_schema() AS schema,
		host(inet_server_addr()) AS address,
		inet_server_port() AS port`).Scan(&print).Error
	if err != nil {
		return nil, err
	}
	return &print, nil
}

func sameDatabase(a, b databaseFingerprint) bool {
	if a.Database != b.Database || a.Schema != b.Schema {
		return false
	}
	// A Unix socket reports no address or port; two such connections to the same
	// database name are the same server.
	return equalOptional(a.Address, b.Address) && equalOptionalInt(a.Port, b.Port)
}

func equalOptional(a, b *string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func equalOptionalInt(a, b *int) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

// assertUpstreamSource checks the source carries the upstream panel tables and
// not the GoWave schema, so a GoWave database is never copied onto itself.
func assertUpstreamSource(sourceTables []string) error {
	present := make(map[string]struct{}, len(sourceTables))
	for _, table := range sourceTables {
		present[table] = struct{}{}
	}
	if _, ok := present[migrationsTable]; ok {
		return fmt.Errorf("%w; to copy one GoWave database into another use pg_dump, or migrate sqlite-to-pg from a SQLite install", ErrSourceIsGoWave)
	}
	var missing []string
	for table := range requiredUpstreamTables {
		if _, ok := present[table]; !ok {
			missing = append(missing, table)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("source does not look like an upstream panel database; missing tables: %s", strings.Join(missing, ", "))
	}
	return nil
}

// copyPostgresDatabase clears the target tables once up front and reloads them
// in foreign-key order, so constraints stay enforced for the whole copy.
func copyPostgresDatabase(source, target *gorm.DB, sourceTables []string, report *PGToPGReport) error {
	targetTables, err := listPostgresCurrentTables(target)
	if err != nil {
		return fmt.Errorf("list target tables: %w", err)
	}
	order, err := postgresForeignKeyOrder(target, targetTables)
	if err != nil {
		return err
	}

	sourceSet := make(map[string]struct{}, len(sourceTables))
	for _, table := range sourceTables {
		sourceSet[table] = struct{}{}
	}

	var targets []string
	for _, table := range sourceTables {
		switch {
		case table == migrationsTable:
		case order[table] == 0:
			report.SkippedTables = append(report.SkippedTables, table)
		default:
			targets = append(targets, table)
		}
	}
	sort.SliceStable(targets, func(i, j int) bool { return order[targets[i]] < order[targets[j]] })

	for _, table := range targetTables {
		if table == migrationsTable {
			continue
		}
		if _, ok := sourceSet[table]; !ok {
			report.EmptyTargetTables = append(report.EmptyTargetTables, table)
		}
	}
	sort.Strings(report.EmptyTargetTables)

	// Every table the GoWave schema owns is cleared, not only the ones the
	// source fills: CASCADE would otherwise wipe rows copied earlier.
	clear := make([]string, 0, len(targetTables))
	for _, table := range targetTables {
		if table == migrationsTable {
			continue
		}
		clear = append(clear, quoteIdentifier(table))
	}
	if len(clear) > 0 {
		if err := target.Exec("TRUNCATE " + strings.Join(clear, ", ") + " RESTART IDENTITY CASCADE").Error; err != nil {
			return fmt.Errorf("clear target tables: %w", err)
		}
	}

	fmt.Printf("[*] Copying %d tables into the GoWave schema...\n", len(targets))
	for _, table := range targets {
		count, normalized, dropped, err := copyPostgresTable(source, target, table)
		if err != nil {
			return fmt.Errorf("migrate table %s: %w", table, err)
		}
		report.Rows[table] = count
		report.NormalizedNulls += normalized
		report.DroppedColumns = append(report.DroppedColumns, dropped...)
		fmt.Printf("  -> %-36s %d rows\n", table, count)
	}

	fmt.Println("[*] Resetting PostgreSQL sequence IDs...")
	if err := resetPostgresSequences(target); err != nil {
		return fmt.Errorf("reset sequences: %w", err)
	}
	return nil
}

// copyPostgresTable copies the columns the two schemas share. It returns the
// copied row count, how many NULLs became a Go zero value, and the source
// columns GoWave does not store.
func copyPostgresTable(source, target *gorm.DB, table string) (int64, int64, []string, error) {
	sourceColumns, err := listPostgresColumns(source, table)
	if err != nil {
		return 0, 0, nil, fmt.Errorf("read source columns: %w", err)
	}
	targetColumns, err := listPostgresColumns(target, table)
	if err != nil {
		return 0, 0, nil, fmt.Errorf("read target columns: %w", err)
	}
	targetByName := make(map[string]postgresColumn, len(targetColumns))
	for _, column := range targetColumns {
		targetByName[strings.ToLower(column.Name)] = column
	}

	var (
		selects      []string
		names        []string
		placeholders []string
		common       []postgresColumn
		dropped      []string
	)
	for _, column := range sourceColumns {
		targetColumn, ok := targetByName[strings.ToLower(column.Name)]
		if !ok {
			dropped = append(dropped, table+"."+column.Name)
			continue
		}
		quoted := quoteIdentifier(targetColumn.Name)
		selects = append(selects, postgresSelectExpression(column))
		names = append(names, quoted)
		placeholders = append(placeholders, postgresPlaceholder(targetColumn))
		common = append(common, targetColumn)
	}
	if len(common) == 0 {
		return 0, 0, dropped, nil
	}

	selectSQL := "SELECT " + strings.Join(selects, ", ") + " FROM " + quoteIdentifier(table)
	rows, err := source.Raw(selectSQL).Rows()
	if err != nil {
		return 0, 0, dropped, err
	}
	defer rows.Close()

	insertPrefix := "INSERT INTO " + quoteIdentifier(table) + " (" + strings.Join(names, ", ") + ") VALUES "
	rowPlaceholder := "(" + strings.Join(placeholders, ", ") + ")"
	batchRows := min(pgToPGBatchRows, pgToPGMaxBindParams/len(common))
	if batchRows < 1 {
		batchRows = 1
	}
	batch := make([]interface{}, 0, batchRows*len(common))
	pending := 0
	inserted := int64(0)
	normalized := int64(0)

	flush := func() error {
		if pending == 0 {
			return nil
		}
		valuesSQL := strings.TrimSuffix(strings.Repeat(rowPlaceholder+", ", pending), ", ")
		if err := target.Exec(insertPrefix+valuesSQL, batch...).Error; err != nil {
			return fmt.Errorf("insert rows %d-%d: %w", inserted+1, inserted+int64(pending), err)
		}
		inserted += int64(pending)
		batch = batch[:0]
		pending = 0
		return nil
	}

	for rows.Next() {
		values := make([]interface{}, len(common))
		scanTargets := make([]interface{}, len(common))
		for i := range values {
			scanTargets[i] = &values[i]
		}
		if err := rows.Scan(scanTargets...); err != nil {
			return inserted, normalized, dropped, err
		}
		for i, value := range values {
			column := common[i]
			if value == nil {
				if zero, ok := database.ImportedZeroValue(table, column.Name); ok {
					values[i] = zero
					normalized++
				}
				continue
			}
			if raw, ok := value.([]byte); ok && column.DataType != "bytea" {
				value = string(raw)
				values[i] = value
			}
			if text, ok := value.(string); ok && text == "" && postgresRejectsEmptyText(column) {
				values[i] = nil
			}
		}
		batch = append(batch, values...)
		pending++
		if pending == batchRows {
			if err := flush(); err != nil {
				return inserted, normalized, dropped, err
			}
		}
	}
	if err := rows.Err(); err != nil {
		return inserted, normalized, dropped, err
	}
	if err := rows.Close(); err != nil {
		return inserted, normalized, dropped, err
	}
	if err := flush(); err != nil {
		return inserted, normalized, dropped, err
	}

	var sourceCount int64
	if err := source.Table(table).Count(&sourceCount).Error; err != nil {
		return inserted, normalized, dropped, err
	}
	if sourceCount != inserted {
		return inserted, normalized, dropped, fmt.Errorf("row count mismatch: source=%d copied=%d", sourceCount, inserted)
	}
	return inserted, normalized, dropped, nil
}

func listPostgresColumns(db *gorm.DB, table string) ([]postgresColumn, error) {
	var columns []postgresColumn
	err := db.Raw(`
		SELECT column_name, data_type, udt_name
		FROM information_schema.columns
		WHERE table_schema = current_schema() AND table_name = ?
		ORDER BY ordinal_position`, table).Scan(&columns).Error
	return columns, err
}

// postgresSelectExpression reads values the pgx driver cannot hand to another
// connection unchanged — arrays, enums, UUIDs, JSON and numerics — as text.
// PostgreSQL parses each of those text forms back on insert.
func postgresSelectExpression(column postgresColumn) string {
	quoted := quoteIdentifier(column.Name)
	if postgresNeedsTextTransfer(column) {
		return "CAST(" + quoted + " AS text) AS " + quoted
	}
	return quoted
}

func postgresNeedsTextTransfer(column postgresColumn) bool {
	if column.DataType == "ARRAY" || strings.HasPrefix(column.UdtName, "_") {
		return true
	}
	switch column.DataType {
	case "uuid", "json", "jsonb", "numeric", "money", "interval",
		"inet", "cidr", "macaddr", "macaddr8", "xml", "bit", "bit varying",
		"USER-DEFINED":
		return true
	}
	return false
}

// postgresPlaceholder casts a text value back into the target column's type.
func postgresPlaceholder(column postgresColumn) string {
	if typeName := postgresCastType(column); typeName != "" {
		return "CAST(? AS " + typeName + ")"
	}
	return "?"
}

func postgresCastType(column postgresColumn) string {
	if column.DataType == "ARRAY" || strings.HasPrefix(column.UdtName, "_") {
		return quoteIdentifier(strings.TrimPrefix(column.UdtName, "_")) + "[]"
	}
	switch column.DataType {
	case "uuid", "json", "jsonb", "numeric", "money", "interval",
		"inet", "cidr", "macaddr", "macaddr8", "xml":
		return column.DataType
	case "bit", "bit varying":
		return column.DataType
	case "USER-DEFINED":
		return quoteIdentifier(column.UdtName)
	}
	return ""
}

// postgresRejectsEmptyText reports whether "" would fail the target column's
// input parser; upstream rows store it where GoWave keeps NULL.
func postgresRejectsEmptyText(column postgresColumn) bool {
	switch column.DataType {
	case "uuid", "json", "jsonb", "numeric", "money", "interval",
		"inet", "cidr", "macaddr", "macaddr8", "xml":
		return true
	}
	return false
}

func printPGToPGReport(report *PGToPGReport) {
	if len(report.SkippedTables) > 0 {
		sort.Strings(report.SkippedTables)
		fmt.Printf("[i] %d source tables have no GoWave counterpart and were skipped: %s\n",
			len(report.SkippedTables), strings.Join(report.SkippedTables, ", "))
	}
	if len(report.DroppedColumns) > 0 {
		sort.Strings(report.DroppedColumns)
		fmt.Printf("[i] %d source columns are not stored by GoWave: %s\n",
			len(report.DroppedColumns), strings.Join(report.DroppedColumns, ", "))
	}
	if len(report.EmptyTargetTables) > 0 {
		fmt.Printf("[i] %d GoWave tables have no source table and stay empty: %s\n",
			len(report.EmptyTargetTables), strings.Join(report.EmptyTargetTables, ", "))
	}
	if report.NormalizedNulls > 0 {
		fmt.Printf("[i] %d NULL values in non-pointer Go model fields were mapped to their Go zero values.\n", report.NormalizedNulls)
	}
	var total int64
	for _, count := range report.Rows {
		total += count
	}
	fmt.Printf("\n[✔] %d rows copied into the GoWave PostgreSQL database.\n", total)
	fmt.Println("Set DB_DRIVER=postgres and DATABASE_URL to the target DSN, then start GoWave and verify the panel.")
	fmt.Println("The upstream panel database was only read; stop the old panel once GoWave looks correct.")
}

// MigratePGToPGInteractive asks for both DSNs and runs the migration.
func MigratePGToPGInteractive(cfg *config.Config, reader *bufio.Reader) {
	fmt.Println("\n=============================================================")
	fmt.Println("  Migrate upstream panel PostgreSQL to GoWave PostgreSQL     ")
	fmt.Println("=============================================================")

	detectedSource, _ := DetectUpstreamPostgresDSN()
	if detectedSource != "" {
		fmt.Printf("[*] Found an upstream panel PostgreSQL configuration: %s\n", redactDSN(detectedSource))
		fmt.Print("Press ENTER to use it, or type a custom source DSN: ")
	} else {
		fmt.Print("Enter the upstream panel PostgreSQL DSN: ")
	}
	input, _ := reader.ReadString('\n')
	sourceDSN := strings.TrimSpace(input)
	if sourceDSN == "" {
		sourceDSN = detectedSource
	}

	detectedTarget, _ := DetectPostgresTargetDSN(cfg)
	if detectedTarget != "" {
		fmt.Printf("[*] GoWave PostgreSQL target: %s\n", redactDSN(detectedTarget))
		fmt.Print("Press ENTER to use it, or type a custom target DSN: ")
	} else {
		fmt.Print("Enter the GoWave PostgreSQL DSN: ")
	}
	input, _ = reader.ReadString('\n')
	targetDSN := strings.TrimSpace(input)
	if targetDSN == "" {
		targetDSN = detectedTarget
	}

	fmt.Print("Replace existing rows in the target if it is not empty? [y/N]: ")
	input, _ = reader.ReadString('\n')
	force := strings.EqualFold(strings.TrimSpace(input), "y")

	if _, err := MigratePostgresToPostgres(cfg, sourceDSN, targetDSN, force); err != nil {
		fmt.Printf("[-] Migration failed: %v\n", err)
		return
	}
}
