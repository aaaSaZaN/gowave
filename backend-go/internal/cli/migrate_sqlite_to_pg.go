package cli

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"gowave/config"
	"gowave/internal/database"
)

// ErrPostgresTargetNotEmpty is returned when the destination already holds
// users, nodes or hosts and the caller did not ask to overwrite them.
var ErrPostgresTargetNotEmpty = errors.New("target PostgreSQL database already contains data")

// sqliteToPGBatchRows bounds a multi-row INSERT; the column count further
// limits it so a batch stays under PostgreSQL's 65535 bind parameters.
const sqliteToPGBatchRows = 500

// migrationsTable records applied schema files per database, so the target
// keeps its own rows instead of the source's.
const migrationsTable = "gowave_migrations"

// DetectGoWaveSQLitePath attempts to locate an existing GoWave SQLite database file.
func DetectGoWaveSQLitePath(cfg *config.Config) (string, error) {
	if cfg != nil && cfg.DBDriver == "sqlite" && strings.TrimSpace(cfg.DatabaseURL) != "" {
		if fi, err := os.Stat(cfg.DatabaseURL); err == nil && !fi.IsDir() {
			return cfg.DatabaseURL, nil
		}
	}

	candidates := []string{
		"gowave.db",
		"gowave-migrated.sqlite",
		"/opt/gowave/gowave.db",
		"data/gowave.db",
	}

	for _, path := range candidates {
		if fi, err := os.Stat(path); err == nil && !fi.IsDir() {
			return path, nil
		}
	}

	return "", fmt.Errorf("no existing GoWave SQLite database found automatically; please specify the path")
}

// DetectPostgresTargetDSN finds the PostgreSQL DSN from the config, the
// environment or the POSTGRES_* variables used by docker-compose.yml.
func DetectPostgresTargetDSN(cfg *config.Config) (string, error) {
	if cfg != nil && cfg.DBDriver == "postgres" && strings.TrimSpace(cfg.DatabaseURL) != "" {
		return cfg.DatabaseURL, nil
	}
	if envDSN := os.Getenv("DATABASE_URL"); strings.TrimSpace(envDSN) != "" {
		return envDSN, nil
	}
	if envDSN := os.Getenv("POSTGRES_DSN"); strings.TrimSpace(envDSN) != "" {
		return envDSN, nil
	}

	pass := os.Getenv("POSTGRES_PASSWORD")
	if pass == "" {
		return "", errors.New("no PostgreSQL target configured; set DATABASE_URL or POSTGRES_PASSWORD, or pass the DSN explicitly")
	}
	user := os.Getenv("POSTGRES_USER")
	if user == "" {
		user = "gowave"
	}
	db := os.Getenv("POSTGRES_DB")
	if db == "" {
		db = "gowave"
	}
	port := os.Getenv("POSTGRES_PORT")
	if port == "" {
		port = "5432"
	}

	dsn := url.URL{
		Scheme:   "postgresql",
		User:     url.UserPassword(user, pass),
		Host:     "127.0.0.1:" + port,
		Path:     "/" + db,
		RawQuery: "sslmode=disable",
	}
	return dsn.String(), nil
}

var dsnPasswordPattern = regexp.MustCompile(`(?i)(password\s*=\s*)('[^']*'|\S+)`)

// redactDSN hides the password of a URL or key=value PostgreSQL DSN.
func redactDSN(dsn string) string {
	if parsed, err := url.Parse(dsn); err == nil && parsed.Scheme != "" && parsed.Host != "" {
		return parsed.Redacted()
	}
	return dsnPasswordPattern.ReplaceAllString(dsn, "${1}xxxxx")
}

// MigrateSQLiteToPostgres copies all tables from a GoWave SQLite database into
// PostgreSQL in one transaction. Tables are cleared once up front and loaded
// in foreign-key order, so constraints stay enforced and no superuser role is
// needed. A target that already has users, nodes or hosts is only replaced
// when force is set; otherwise ErrPostgresTargetNotEmpty is returned.
func MigrateSQLiteToPostgres(_ *config.Config, sourcePath, targetDSN string, force bool) error {
	if strings.TrimSpace(sourcePath) == "" {
		return errors.New("source SQLite database path cannot be empty")
	}
	if strings.TrimSpace(targetDSN) == "" {
		return errors.New("target PostgreSQL DSN cannot be empty")
	}

	absSource, err := filepath.Abs(sourcePath)
	if err != nil {
		return fmt.Errorf("resolve SQLite path: %w", err)
	}
	if fi, err := os.Stat(absSource); err != nil || fi.IsDir() {
		return fmt.Errorf("source SQLite database not found: %s", absSource)
	}

	fmt.Printf("[*] Opening SQLite database: %s\n", absSource)
	sqliteDB, err := gorm.Open(sqlite.Open(absSource), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return fmt.Errorf("connect to SQLite: %w", err)
	}
	defer closeGormDB(sqliteDB)

	// PostgreSQL enforces foreign keys while loading; report broken rows by
	// name instead of failing halfway through a table.
	if err := checkSQLiteForeignKeys(sqliteDB); err != nil {
		return err
	}

	fmt.Printf("[*] Connecting to PostgreSQL destination: %s\n", redactDSN(targetDSN))
	pgDB, err := gorm.Open(postgres.Open(targetDSN), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return fmt.Errorf("connect to PostgreSQL: %w", err)
	}
	defer closeGormDB(pgDB)

	fmt.Println("[*] Applying GoWave schema to PostgreSQL...")
	if err := database.MigrateSchema(pgDB); err != nil {
		return fmt.Errorf("initialize PostgreSQL schema: %w", err)
	}

	if !force {
		existing, err := postgresTargetData(pgDB)
		if err != nil {
			return fmt.Errorf("inspect target database: %w", err)
		}
		if len(existing) > 0 {
			return fmt.Errorf("%w (%s); rerun with --force to replace it", ErrPostgresTargetNotEmpty, strings.Join(existing, ", "))
		}
	}

	sqliteTables, err := listSQLiteTables(sqliteDB)
	if err != nil {
		return fmt.Errorf("list SQLite tables: %w", err)
	}

	err = pgDB.Transaction(func(tx *gorm.DB) error {
		pgTables, err := listPostgresCurrentTables(tx)
		if err != nil {
			return fmt.Errorf("list PostgreSQL tables: %w", err)
		}
		order, err := postgresForeignKeyOrder(tx, pgTables)
		if err != nil {
			return err
		}

		var targets, skipped []string
		for _, table := range sqliteTables {
			switch {
			case table == migrationsTable:
			case order[table] == 0:
				skipped = append(skipped, table)
			default:
				targets = append(targets, table)
			}
		}
		sort.SliceStable(targets, func(i, j int) bool { return order[targets[i]] < order[targets[j]] })

		if len(targets) > 0 {
			quoted := make([]string, len(targets))
			for i, table := range targets {
				quoted[i] = quoteIdentifier(table)
			}
			if err := tx.Exec("TRUNCATE " + strings.Join(quoted, ", ") + " RESTART IDENTITY CASCADE").Error; err != nil {
				return fmt.Errorf("clear target tables: %w", err)
			}
		}

		fmt.Printf("[*] Migrating %d tables from SQLite to PostgreSQL...\n", len(targets))
		for _, table := range targets {
			count, err := migrateTableSQLiteToPG(sqliteDB, tx, table)
			if err != nil {
				return fmt.Errorf("migrate table %s: %w", table, err)
			}
			fmt.Printf("  -> %-36s %d rows\n", table, count)
		}
		for _, table := range skipped {
			fmt.Printf("  -> %-36s skipped (no such table in target)\n", table)
		}

		fmt.Println("[*] Resetting PostgreSQL sequence IDs...")
		if err := resetPostgresSequences(tx); err != nil {
			return fmt.Errorf("reset sequences: %w", err)
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("copy data to PostgreSQL: %w", err)
	}

	fmt.Println("\n[✔] Data migration completed successfully!")
	if os.Getenv("IN_DOCKER") == "" {
		fmt.Println("To run GoWave with PostgreSQL, update your .env or docker-compose.yml:")
		fmt.Printf("  DB_DRIVER=postgres\n  DATABASE_URL=%s  (password hidden)\n", redactDSN(targetDSN))
	}
	return nil
}

func checkSQLiteForeignKeys(db *gorm.DB) error {
	var violations []struct {
		Table  string `gorm:"column:table"`
		RowID  *int64 `gorm:"column:rowid"`
		Parent string `gorm:"column:parent"`
	}
	if err := db.Raw("PRAGMA foreign_key_check").Scan(&violations).Error; err != nil {
		return fmt.Errorf("check SQLite foreign keys: %w", err)
	}
	if len(violations) == 0 {
		return nil
	}
	const shown = 10
	var lines []string
	for i, v := range violations {
		if i == shown {
			lines = append(lines, fmt.Sprintf("  ... and %d more", len(violations)-shown))
			break
		}
		row := "?"
		if v.RowID != nil {
			row = fmt.Sprintf("%d", *v.RowID)
		}
		lines = append(lines, fmt.Sprintf("  %s rowid %s references a missing %s row", v.Table, row, v.Parent))
	}
	return fmt.Errorf("source SQLite has %d rows pointing to deleted records; PostgreSQL would reject them:\n%s\nDelete or fix these rows in the source database, then retry",
		len(violations), strings.Join(lines, "\n"))
}

// postgresTargetData lists the core tables that already hold rows.
func postgresTargetData(db *gorm.DB) ([]string, error) {
	var found []string
	for _, table := range []string{"users", "nodes", "hosts"} {
		var count int64
		if err := db.Table(table).Count(&count).Error; err != nil {
			return nil, err
		}
		if count > 0 {
			found = append(found, fmt.Sprintf("%s: %d", table, count))
		}
	}
	return found, nil
}

func listSQLiteTables(db *gorm.DB) ([]string, error) {
	var tables []string
	query := `SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name`
	if err := db.Raw(query).Scan(&tables).Error; err != nil {
		return nil, err
	}
	return tables, nil
}

func listPostgresCurrentTables(db *gorm.DB) ([]string, error) {
	var tables []string
	err := db.Raw(`
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = current_schema() AND table_type = 'BASE TABLE'
		ORDER BY table_name`).Scan(&tables).Error
	return tables, err
}

// postgresForeignKeyOrder ranks tables so every referenced table precedes the
// tables pointing at it. Ranks start at 1; tables outside the target are 0.
func postgresForeignKeyOrder(db *gorm.DB, tables []string) (map[string]int, error) {
	var edges []struct {
		Child  string `gorm:"column:child"`
		Parent string `gorm:"column:parent"`
	}
	if err := db.Raw(`
		SELECT DISTINCT child.relname AS child, parent.relname AS parent
		FROM pg_constraint c
		JOIN pg_class child ON child.oid = c.conrelid
		JOIN pg_class parent ON parent.oid = c.confrelid
		WHERE c.contype = 'f'
		  AND child.relnamespace = current_schema()::text::regnamespace
		  AND parent.relnamespace = current_schema()::text::regnamespace
		  AND child.oid <> parent.oid`).Scan(&edges).Error; err != nil {
		return nil, fmt.Errorf("read foreign keys: %w", err)
	}

	pending := make(map[string]int, len(tables))
	children := make(map[string][]string)
	for _, table := range tables {
		pending[table] = 0
	}
	for _, edge := range edges {
		pending[edge.Child]++
		children[edge.Parent] = append(children[edge.Parent], edge.Child)
	}

	order := make(map[string]int, len(tables))
	var ready []string
	for _, table := range tables {
		if pending[table] == 0 {
			ready = append(ready, table)
		}
	}
	for len(ready) > 0 {
		sort.Strings(ready)
		table := ready[0]
		ready = ready[1:]
		order[table] = len(order) + 1
		for _, child := range children[table] {
			pending[child]--
			if pending[child] == 0 {
				ready = append(ready, child)
			}
		}
	}
	if len(order) != len(tables) {
		var cyclic []string
		for _, table := range tables {
			if order[table] == 0 {
				cyclic = append(cyclic, table)
			}
		}
		return nil, fmt.Errorf("foreign keys form a cycle between tables: %s", strings.Join(cyclic, ", "))
	}
	return order, nil
}

func migrateTableSQLiteToPG(source, dest *gorm.DB, table string) (int64, error) {
	// Query columns existing in PostgreSQL
	var pgCols []postgresColumn
	if err := dest.Raw(`
		SELECT column_name, data_type, udt_name
		FROM information_schema.columns
		WHERE table_schema = current_schema() AND table_name = ?
		ORDER BY ordinal_position`, table).Scan(&pgCols).Error; err != nil {
		return 0, err
	}

	pgColMap := make(map[string]postgresColumn, len(pgCols))
	for _, c := range pgCols {
		pgColMap[strings.ToLower(c.Name)] = c
	}

	// Query SQLite columns
	var sqliteCols []struct {
		Name string `gorm:"column:name"`
	}
	if err := source.Raw("PRAGMA table_info(" + quoteIdentifier(table) + ")").Scan(&sqliteCols).Error; err != nil {
		return 0, err
	}

	// Calculate common columns
	var commonCols []postgresColumn
	for _, sc := range sqliteCols {
		if pc, exists := pgColMap[strings.ToLower(sc.Name)]; exists {
			commonCols = append(commonCols, pc)
		}
	}
	if len(commonCols) == 0 {
		return 0, nil
	}

	// Build SELECT from SQLite
	selectNames := make([]string, len(commonCols))
	colNames := make([]string, len(commonCols))
	placeholders := make([]string, len(commonCols))

	for i, c := range commonCols {
		quoted := quoteIdentifier(c.Name)
		selectNames[i] = quoted
		colNames[i] = quoted
		switch {
		case c.DataType == "json" || c.DataType == "jsonb":
			placeholders[i] = "CAST(? AS jsonb)"
		case c.DataType == "uuid":
			placeholders[i] = "CAST(? AS uuid)"
		default:
			placeholders[i] = "?"
		}
	}
	rowPlaceholder := "(" + strings.Join(placeholders, ", ") + ")"

	selectSQL := fmt.Sprintf("SELECT %s FROM %s", strings.Join(selectNames, ", "), quoteIdentifier(table))
	rows, err := source.Raw(selectSQL).Rows()
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	insertPrefix := fmt.Sprintf("INSERT INTO %s (%s) VALUES ", quoteIdentifier(table), strings.Join(colNames, ", "))
	batchRows := min(sqliteToPGBatchRows, 65535/len(commonCols))
	batch := make([]interface{}, 0, batchRows*len(commonCols))
	pendingRows := 0

	inserted := int64(0)
	flush := func() error {
		if pendingRows == 0 {
			return nil
		}
		valuesSQL := strings.TrimSuffix(strings.Repeat(rowPlaceholder+", ", pendingRows), ", ")
		if err := dest.Exec(insertPrefix+valuesSQL, batch...).Error; err != nil {
			return fmt.Errorf("insert rows %d-%d: %w", inserted+1, inserted+int64(pendingRows), err)
		}
		inserted += int64(pendingRows)
		batch = batch[:0]
		pendingRows = 0
		return nil
	}

	for rows.Next() {
		values := make([]interface{}, len(commonCols))
		destinations := make([]interface{}, len(commonCols))
		for i := range values {
			destinations[i] = &values[i]
		}
		if err := rows.Scan(destinations...); err != nil {
			return inserted, err
		}

		for i, val := range values {
			col := commonCols[i]
			if val == nil {
				if zero, ok := database.ImportedZeroValue(table, col.Name); ok {
					values[i] = zero
				}
				continue
			}

			// Format booleans properly for PostgreSQL driver
			if col.DataType == "boolean" {
				switch v := val.(type) {
				case int64:
					values[i] = (v != 0)
				case int:
					values[i] = (v != 0)
				case string:
					values[i] = (v == "1" || strings.EqualFold(v, "true"))
				}
			}

			// Convert byte slice to string for text/json columns
			if bytes, ok := val.([]byte); ok && col.DataType != "bytea" {
				valStr := string(bytes)
				if (col.DataType == "json" || col.DataType == "jsonb") && valStr == "" {
					values[i] = nil
				} else {
					values[i] = valStr
				}
			} else if str, ok := val.(string); ok {
				if (col.DataType == "json" || col.DataType == "jsonb") && str == "" {
					values[i] = nil
				}
				if col.DataType == "uuid" && str == "" {
					values[i] = nil
				}
			}
		}

		batch = append(batch, values...)
		pendingRows++
		if pendingRows == batchRows {
			if err := flush(); err != nil {
				return inserted, err
			}
		}
	}
	if err := rows.Err(); err != nil {
		return inserted, err
	}
	return inserted, flush()
}

func resetPostgresSequences(db *gorm.DB) error {
	const query = `
DO $$
DECLARE
    tbl text;
    col text;
    seq text;
BEGIN
    FOR tbl, col IN
        SELECT table_name, column_name
        FROM information_schema.columns
        WHERE table_schema = current_schema()
          AND column_default LIKE 'nextval%'
    LOOP
        seq := pg_get_serial_sequence(quote_ident(tbl), col);
        IF seq IS NOT NULL THEN
            EXECUTE format('SELECT setval(%L, COALESCE(MAX(%I), 1), MAX(%I) IS NOT NULL) FROM %I', seq, col, col, tbl);
        END IF;
    END LOOP;
END $$;`
	return db.Exec(query).Error
}
