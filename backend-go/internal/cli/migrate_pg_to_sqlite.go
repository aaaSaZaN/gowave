package cli

import (
	"bufio"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"gowave/config"
	"gowave/internal/database"
)

var requiredUpstreamTables = map[string]bool{
	"users":                   true,
	"user_traffic":            true,
	"nodes":                   true,
	"hosts":                   true,
	"config_profiles":         true,
	"config_profile_inbounds": true,
	"internal_squads":         true,
	"internal_squad_members":  true,
	"internal_squad_inbounds": true,
	"subscription_templates":  true,
	"subscription_settings":   true,
}

type postgresColumn struct {
	Name     string `gorm:"column:column_name"`
	DataType string `gorm:"column:data_type"`
	UdtName  string `gorm:"column:udt_name"`
}

type postgresTable struct {
	Name string `gorm:"column:table_name"`
}

type sqliteColumn struct {
	Name string `gorm:"column:name"`
}

func DetectUpstreamPostgresDSN() (string, error) {
	candidates := []string{
		"/opt/remnawave/.env",
		"../backend-ts/.env",
		"../.env",
		".env",
	}

	for _, path := range candidates {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		lines := strings.Split(string(data), "\n")
		var user, pass, dbName, fullURL, directURL string
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "#") || !strings.Contains(line, "=") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			key := strings.TrimSpace(parts[0])
			value := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
			switch key {
			case "DATABASE_URL":
				fullURL = value
			case "DIRECT_URL":
				directURL = value
			case "POSTGRES_USER":
				user = value
			case "POSTGRES_PASSWORD":
				pass = value
			case "POSTGRES_DB":
				dbName = value
			}
		}

		panelEnv := isUpstreamEnvPath(path)
		for _, candidate := range []string{directURL, fullURL} {
			if candidate == "" {
				continue
			}
			parsed, err := url.Parse(candidate)
			if err != nil || (parsed.Scheme != "postgres" && parsed.Scheme != "postgresql") {
				continue
			}
			if !panelEnv && !strings.EqualFold(parsed.Hostname(), "remnawave-db") {
				continue
			}
			if strings.EqualFold(parsed.Hostname(), "remnawave-db") {
				parsed.Host = "127.0.0.1:6767"
			}
			return parsed.String(), nil
		}

		if panelEnv && pass != "" {
			if user == "" {
				user = "postgres"
			}
			if dbName == "" {
				dbName = "postgres"
			}
			return (&url.URL{
				Scheme:   "postgresql",
				User:     url.UserPassword(user, pass),
				Host:     "127.0.0.1:6767",
				Path:     "/" + dbName,
				RawQuery: "sslmode=disable",
			}).String(), nil
		}
	}

	return "", fmt.Errorf("no upstream panel PostgreSQL configuration automatically detected")
}

func isUpstreamEnvPath(path string) bool {
	cleanPath := filepath.ToSlash(filepath.Clean(path))
	return cleanPath == "/opt/remnawave/.env" ||
		strings.HasPrefix(cleanPath, "../backend-ts/") ||
		strings.Contains(cleanPath, "/backend-ts/") ||
		strings.Contains(cleanPath, "/remnawave/")
}

// MigratePostgresToSQLite reads the source in a read-only transaction and builds
// a fresh SQLite database beside the requested destination. It never rewrites
// the source DB, GoWave .env, or upstream containers.
func MigratePostgresToSQLite(_ *config.Config, sourceDSN, destPath string) error {
	if sourceDSN == "" {
		detected, err := DetectUpstreamPostgresDSN()
		if err != nil {
			return err
		}
		sourceDSN = detected
	}
	if destPath == "" {
		destPath = "gowave-migrated.sqlite"
	}

	absDestPath, err := filepath.Abs(destPath)
	if err != nil {
		return fmt.Errorf("resolve SQLite destination path: %w", err)
	}
	if _, err := os.Stat(absDestPath); err == nil {
		return fmt.Errorf("destination already exists; choose a new path to avoid overwriting it: %s", absDestPath)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("check SQLite destination: %w", err)
	}

	tmpFile, err := os.CreateTemp(filepath.Dir(absDestPath), ".gowave-migration-*.sqlite")
	if err != nil {
		return fmt.Errorf("create temporary SQLite destination: %w", err)
	}
	tmpPath := tmpFile.Name()
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close temporary SQLite destination: %w", err)
	}
	published := false
	defer func() {
		if !published {
			_ = os.Remove(tmpPath)
		}
	}()

	fmt.Println("[*] Connecting to PostgreSQL source (read-only)...")
	pgDB, err := gorm.Open(postgres.Open(sourceDSN), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return fmt.Errorf("connect to PostgreSQL: %w", err)
	}
	defer closeGormDB(pgDB)

	fmt.Println("[*] Creating SQLite schema...")
	sqliteDB, err := gorm.Open(sqlite.Open(tmpPath), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return fmt.Errorf("open temporary SQLite database: %w", err)
	}
	defer closeGormDB(sqliteDB)
	if sqlDB, err := sqliteDB.DB(); err == nil {
		sqlDB.SetMaxOpenConns(1)
		sqlDB.SetMaxIdleConns(1)
	}
	if err := sqliteDB.Exec("PRAGMA foreign_keys = OFF").Error; err != nil {
		return fmt.Errorf("disable SQLite foreign keys during import: %w", err)
	}
	if err := database.MigrateSchema(sqliteDB); err != nil {
		return fmt.Errorf("create GoWave SQLite schema: %w", err)
	}

	var normalizedNulls int64
	err = pgDB.Transaction(func(source *gorm.DB) error {
		return sqliteDB.Transaction(func(destination *gorm.DB) error {
			tables, err := listPostgresTables(source)
			if err != nil {
				return fmt.Errorf("list PostgreSQL tables: %w", err)
			}
			tableSet := make(map[string]struct{}, len(tables))
			for _, table := range tables {
				tableSet[table] = struct{}{}
			}
			for table := range requiredUpstreamTables {
				if _, ok := tableSet[table]; !ok {
					return fmt.Errorf("required upstream panel table %q does not exist", table)
				}
			}

			for _, table := range tables {
				count, normalized, found, err := migratePostgresTable(source, destination, table)
				if err != nil {
					return fmt.Errorf("migrate table %s: %w", table, err)
				}
				normalizedNulls += normalized
				if !found {
					return fmt.Errorf("source table %q disappeared during migration", table)
				}
				fmt.Printf("  -> %-36s %d rows\n", table, count)
			}
			return nil
		})
	}, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return fmt.Errorf("copy PostgreSQL data: %w", err)
	}

	if err := sqliteDB.Exec("PRAGMA foreign_keys = ON").Error; err != nil {
		return fmt.Errorf("enable SQLite foreign keys: %w", err)
	}
	var violations []struct {
		Table  string `gorm:"column:table"`
		RowID  *int64 `gorm:"column:rowid"`
		Parent string `gorm:"column:parent"`
		FKID   int    `gorm:"column:fkid"`
	}
	if err := sqliteDB.Raw("PRAGMA foreign_key_check").Scan(&violations).Error; err != nil {
		return fmt.Errorf("validate SQLite foreign keys: %w", err)
	}
	if len(violations) > 0 {
		return fmt.Errorf("SQLite foreign key validation found %d invalid references (first table: %s)", len(violations), violations[0].Table)
	}

	if err := closeGormDB(sqliteDB); err != nil {
		return fmt.Errorf("close SQLite database before publishing: %w", err)
	}
	if err := os.Link(tmpPath, absDestPath); err != nil {
		return fmt.Errorf("publish migrated SQLite database: %w", err)
	}
	published = true
	if err := os.Remove(tmpPath); err != nil {
		return fmt.Errorf("SQLite database was published at %s, but temporary file cleanup failed: %w", absDestPath, err)
	}
	fmt.Printf("\n[✔] PostgreSQL data copied and validated: %s\n", absDestPath)
	fmt.Printf("Set DB_DRIVER=sqlite and DATABASE_URL=%s in the GoWave environment, then restart GoWave.\n", absDestPath)
	if normalizedNulls > 0 {
		fmt.Printf("[i] %d NULL values in non-pointer Go model fields were mapped to their Go zero values.\n", normalizedNulls)
	}
	fmt.Println("The migrated database is ready to inspect before stopping the old upstream panel services.")
	return nil
}

func listPostgresTables(source *gorm.DB) ([]string, error) {
	var tables []postgresTable
	if err := source.Raw(`
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema = 'public' AND table_type = 'BASE TABLE'
		ORDER BY table_name`).Scan(&tables).Error; err != nil {
		return nil, err
	}
	names := make([]string, 0, len(tables))
	for _, table := range tables {
		names = append(names, table.Name)
	}
	return names, nil
}

func migratePostgresTable(source, destination *gorm.DB, table string) (int64, int64, bool, error) {
	var columns []postgresColumn
	if err := source.Raw(`
		SELECT column_name, data_type, udt_name
		FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = ?
		ORDER BY ordinal_position`, table).Scan(&columns).Error; err != nil {
		return 0, 0, false, err
	}
	if len(columns) == 0 {
		return 0, 0, false, nil
	}

	if err := ensureSQLiteTableColumns(destination, table, columns); err != nil {
		return 0, 0, true, err
	}

	selects := make([]string, 0, len(columns))
	columnNames := make([]string, 0, len(columns))
	for _, column := range columns {
		name := quoteIdentifier(column.Name)
		columnNames = append(columnNames, name)
		switch {
		case column.DataType == "ARRAY" || strings.HasPrefix(column.UdtName, "_"):
			selects = append(selects, "to_json("+name+")::text AS "+name)
		case column.DataType == "json" || column.DataType == "jsonb" || column.DataType == "uuid":
			selects = append(selects, "CAST("+name+" AS text) AS "+name)
		default:
			selects = append(selects, name)
		}
	}
	selectSQL := "SELECT " + strings.Join(selects, ", ") + " FROM " + quoteIdentifier("public") + "." + quoteIdentifier(table)
	rows, err := source.Raw(selectSQL).Rows()
	if err != nil {
		return 0, 0, true, err
	}
	defer rows.Close()

	placeholders := make([]string, len(columns))
	for i := range placeholders {
		placeholders[i] = "?"
	}
	insertSQL := "INSERT INTO " + quoteIdentifier(table) + " (" + strings.Join(columnNames, ", ") + ") VALUES (" + strings.Join(placeholders, ", ") + ")"
	inserted := int64(0)
	normalizedNulls := int64(0)
	for rows.Next() {
		values := make([]interface{}, len(columns))
		destinations := make([]interface{}, len(columns))
		for i := range values {
			destinations[i] = &values[i]
		}
		if err := rows.Scan(destinations...); err != nil {
			return inserted, normalizedNulls, true, err
		}
		for i, value := range values {
			if value == nil {
				if zero, ok := database.ImportedZeroValue(table, columns[i].Name); ok {
					values[i] = zero
					normalizedNulls++
				}
				continue
			}
			if bytes, ok := value.([]byte); ok && columns[i].DataType != "bytea" {
				values[i] = string(bytes)
			}
		}
		if err := destination.Exec(insertSQL, values...).Error; err != nil {
			return inserted, normalizedNulls, true, fmt.Errorf("insert row %d: %w", inserted+1, err)
		}
		inserted++
	}
	if err := rows.Err(); err != nil {
		return inserted, normalizedNulls, true, err
	}
	if err := rows.Close(); err != nil {
		return inserted, normalizedNulls, true, err
	}

	var sourceCount, destinationCount int64
	if err := source.Table(table).Count(&sourceCount).Error; err != nil {
		return inserted, normalizedNulls, true, err
	}
	if err := destination.Table(table).Count(&destinationCount).Error; err != nil {
		return inserted, normalizedNulls, true, err
	}
	if sourceCount != inserted || destinationCount != inserted {
		return inserted, normalizedNulls, true, fmt.Errorf("row count mismatch: source=%d copied=%d destination=%d", sourceCount, inserted, destinationCount)
	}
	return inserted, normalizedNulls, true, nil
}

func ensureSQLiteTableColumns(db *gorm.DB, table string, sourceColumns []postgresColumn) error {
	var targetColumns []sqliteColumn
	pragma := "PRAGMA table_info(" + quoteIdentifier(table) + ")"
	if err := db.Raw(pragma).Scan(&targetColumns).Error; err != nil {
		return err
	}

	if len(targetColumns) == 0 {
		definitions := make([]string, 0, len(sourceColumns))
		for _, column := range sourceColumns {
			definitions = append(definitions, quoteIdentifier(column.Name)+" "+sqliteTypeForPostgres(column))
		}
		createSQL := "CREATE TABLE " + quoteIdentifier(table) + " (" + strings.Join(definitions, ", ") + ")"
		if err := db.Exec(createSQL).Error; err != nil {
			return err
		}
		return nil
	}

	existing := make(map[string]struct{}, len(targetColumns))
	for _, column := range targetColumns {
		existing[column.Name] = struct{}{}
	}
	for _, column := range sourceColumns {
		if _, ok := existing[column.Name]; ok {
			continue
		}
		alterSQL := "ALTER TABLE " + quoteIdentifier(table) + " ADD COLUMN " + quoteIdentifier(column.Name) + " " + sqliteTypeForPostgres(column)
		if err := db.Exec(alterSQL).Error; err != nil {
			return fmt.Errorf("add source column %q: %w", column.Name, err)
		}
	}
	return nil
}

func sqliteTypeForPostgres(column postgresColumn) string {
	switch {
	case column.DataType == "ARRAY" || strings.HasPrefix(column.UdtName, "_"):
		return "TEXT"
	case column.DataType == "json" || column.DataType == "jsonb" || column.DataType == "uuid":
		return "TEXT"
	case column.DataType == "boolean":
		return "INTEGER"
	case column.DataType == "smallint" || column.DataType == "integer" || column.DataType == "bigint":
		return "INTEGER"
	case column.DataType == "real" || column.DataType == "double precision":
		return "REAL"
	case column.DataType == "numeric":
		return "NUMERIC"
	case column.DataType == "bytea":
		return "BLOB"
	case strings.HasPrefix(column.DataType, "timestamp") || column.DataType == "date" || strings.HasPrefix(column.DataType, "time"):
		return "DATETIME"
	default:
		return "TEXT"
	}
}

func quoteIdentifier(identifier string) string {
	return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"`
}

func closeGormDB(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

func MigratePGToSQLiteInteractive(cfg *config.Config, reader *bufio.Reader) {
	fmt.Println("\n=======================================================")
	fmt.Println("  Migrate upstream panel PostgreSQL data to GoWave SQLite  ")
	fmt.Println("=======================================================")

	detected, _ := DetectUpstreamPostgresDSN()
	if detected != "" {
		fmt.Println("[*] Found an upstream panel PostgreSQL configuration.")
		fmt.Print("Press ENTER to use it, or type a custom PostgreSQL DSN: ")
	} else {
		fmt.Print("Enter PostgreSQL connection DSN: ")
	}

	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	sourceDSN := detected
	if input != "" {
		sourceDSN = input
	}

	fmt.Print("Enter destination SQLite path [default: gowave-migrated.sqlite]: ")
	destInput, _ := reader.ReadString('\n')
	destPath := strings.TrimSpace(destInput)
	if destPath == "" {
		destPath = "gowave-migrated.sqlite"
	}

	if err := MigratePostgresToSQLite(cfg, sourceDSN, destPath); err != nil {
		fmt.Printf("[-] Migration failed: %v\n", err)
		return
	}
	fmt.Println("[*] The original PostgreSQL database and upstream panel services are untouched.")
	fmt.Println("[*] Point GoWave to the validated SQLite file, restart it, and verify the panel before stopping the upstream panel.")
}
