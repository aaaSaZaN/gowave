package system

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"regexp"
	"remnawave-go/internal/database"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

var restoreIdentifierPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_$]*$`)

type BackupRestoreService struct {
	db *gorm.DB
}

func NewBackupRestoreService(db *gorm.DB) *BackupRestoreService {
	return &BackupRestoreService{db: db}
}

func (s *BackupRestoreService) RestoreFromTarGz(tarGzPath string) error {
	file, err := os.Open(tarGzPath)
	if err != nil {
		return err
	}
	defer file.Close()

	gzr, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA {
			continue
		}
		if strings.HasSuffix(header.Name, ".sql.gz") {
			return s.restoreSqlGz(tr)
		}
	}

	return fmt.Errorf("dump.sql.gz not found in archive")
}

func (s *BackupRestoreService) restoreSqlGz(r io.Reader) error {
	gzr, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gzr.Close()

	// Validate in an isolated database before replacing any live rows. The
	// archive is read-only and both replacement and sequence repair are atomic.
	file, err := os.CreateTemp("", "gowave-restore-*.sqlite")
	if err != nil {
		return err
	}
	path := file.Name()
	file.Close()
	defer os.Remove(path)
	staged, err := database.Init("sqlite", path)
	if err != nil {
		return err
	}
	conn, err := staged.DB()
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := database.MigrateSchema(staged); err != nil {
		return err
	}
	if err := staged.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("PRAGMA defer_foreign_keys = ON").Error; err != nil {
			return err
		}
		return consumePostgresCopyDump(tx, gzr)
	}); err != nil {
		return err
	}
	tables, err := staged.Migrator().GetTables()
	if err != nil {
		return err
	}
	var appTables []string
	for _, table := range tables {
		if table == "sqlite_sequence" || table == "gowave_migrations" {
			continue
		}
		appTables = append(appTables, table)
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		if tx.Dialector.Name() == "sqlite" {
			if err := tx.Exec("PRAGMA defer_foreign_keys = ON").Error; err != nil {
				return err
			}
			for _, table := range appTables {
				if err := tx.Exec(`DELETE FROM "` + table + `"`).Error; err != nil {
					return err
				}
			}
		} else {
			quoted := make([]string, len(appTables))
			for i, table := range appTables {
				quoted[i] = `"` + table + `"`
			}
			if err := tx.Exec("TRUNCATE " + strings.Join(quoted, ",") + " RESTART IDENTITY CASCADE").Error; err != nil {
				return err
			}
		}
		// FK parents precede children in the embedded CREATE TABLE baseline.
		ordered := database.SchemaTableOrder()
		for _, table := range ordered {
			rows, err := staged.Table(table).Rows()
			if err != nil {
				return err
			}
			columns, err := rows.Columns()
			if err != nil {
				rows.Close()
				return err
			}
			quoted, params := make([]string, len(columns)), make([]string, len(columns))
			for i, column := range columns {
				quoted[i] = `"` + column + `"`
				params[i] = "?"
			}
			query := `INSERT INTO "` + table + `" (` + strings.Join(quoted, ",") + `) VALUES (` + strings.Join(params, ",") + `)`
			for rows.Next() {
				values := make([]interface{}, len(columns))
				dest := make([]interface{}, len(columns))
				for i := range values {
					dest[i] = &values[i]
				}
				if err := rows.Scan(dest...); err != nil {
					rows.Close()
					return err
				}
				for i, v := range values {
					if zero, ok := database.ImportedZeroValue(table, columns[i]); ok {
						switch zero.(type) {
						case bool:
							if n, ok := v.(int64); ok {
								values[i] = n != 0
							}
						case time.Time:
							if raw, ok := v.(string); ok {
								values[i], err = parseDumpTime(raw)
								if err != nil {
									rows.Close()
									return err
								}
							}
						}
					}
				}
				if err := tx.Exec(query, values...).Error; err != nil {
					rows.Close()
					return fmt.Errorf("restore %s: %w", table, err)
				}
			}
			err = rows.Err()
			rows.Close()
			if err != nil {
				return err
			}
		}
		if tx.Dialector.Name() == "postgres" {
			for _, table := range ordered {
				if !tx.Migrator().HasColumn(table, "id") {
					continue
				}
				var seq *string
				if err := tx.Raw("SELECT pg_get_serial_sequence(?, 'id')", table).Row().Scan(&seq); err != nil {
					return err
				}
				if seq != nil && *seq != "" {
					if err := tx.Exec(`SELECT setval(?, COALESCE((SELECT MAX(id) FROM "`+table+`"),1), EXISTS(SELECT 1 FROM "`+table+`"))`, *seq).Error; err != nil {
						return err
					}
				}
			}
		}
		return nil
	})
}

func parseDumpTime(raw string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02 15:04:05.999999999Z07:00", "2006-01-02 15:04:05.999999999-07", "2006-01-02 15:04:05.999999999-0700", "2006-01-02 15:04:05.999999999"} {
		if value, err := time.Parse(layout, raw); err == nil {
			return value.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid dump timestamp %q", raw)
}

func consumePostgresCopyDump(tx *gorm.DB, r io.Reader) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 64*1024*1024)

	var currentTable string
	var columns []string
	var skipCurrentTable bool
	var rowNumber int64
	var sawCopy bool
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "COPY ") {
			if currentTable != "" {
				return fmt.Errorf("table %s COPY block did not end before the next block", currentTable)
			}
			table, parsedColumns, err := parseCopyHeader(line)
			if err != nil {
				return err
			}
			currentTable = table
			columns = parsedColumns
			rowNumber = 0
			sawCopy = true
			skipCurrentTable = table == "_prisma_migrations" && !tx.Migrator().HasTable(table)
			if !skipCurrentTable && !tx.Migrator().HasTable(table) {
				return fmt.Errorf("backup contains table %q, which does not exist in the destination database", table)
			}
			continue
		}

		if currentTable == "" {
			continue
		}
		if line == `\.` {
			currentTable = ""
			columns = nil
			skipCurrentTable = false
			continue
		}
		if skipCurrentTable {
			continue
		}

		encodedValues := strings.Split(line, "\t")
		if len(encodedValues) != len(columns) {
			return fmt.Errorf("table %s row %d has %d values for %d columns", currentTable, rowNumber+1, len(encodedValues), len(columns))
		}
		args := make([]interface{}, len(encodedValues))
		for i, value := range encodedValues {
			decoded, err := decodePostgresCopyField(value)
			if err != nil {
				return fmt.Errorf("decode table %s row %d column %s: %w", currentTable, rowNumber+1, columns[i], err)
			}
			if decoded == nil {
				args[i] = nil
				continue
			}
			if currentTable == "passkeys" && columns[i] == "public_key" {
				if raw, ok := decoded.(string); ok && strings.HasPrefix(raw, `\x`) {
					binary, err := hex.DecodeString(strings.TrimPrefix(raw, `\x`))
					if err != nil {
						return fmt.Errorf("decode passkeys public_key at row %d: %w", rowNumber+1, err)
					}
					decoded = binary
				}
			}
			args[i] = decoded
			if zero, ok := database.ImportedZeroValue(currentTable, columns[i]); ok {
				switch zero.(type) {
				case bool:
					if value, ok := decoded.(string); ok {
						parsed, err := strconv.ParseBool(value)
						if err != nil {
							return err
						}
						args[i] = parsed
					}
				case time.Time:
					if raw, ok := decoded.(string); ok {
						parsed, err := parseDumpTime(raw)
						if err != nil {
							return err
						}
						args[i] = parsed
					}
				}
			}
		}

		quotedColumns := make([]string, len(columns))
		placeholders := make([]string, len(columns))
		for i, column := range columns {
			quotedColumns[i] = `"` + strings.ReplaceAll(column, `"`, `""`) + `"`
			placeholders[i] = "?"
		}
		query := `INSERT INTO "` + currentTable + `" (` + strings.Join(quotedColumns, ",") + `) VALUES (` + strings.Join(placeholders, ",") + `)`
		if err := tx.Exec(query, args...).Error; err != nil {
			return fmt.Errorf("insert table %s row %d: %w", currentTable, rowNumber+1, err)
		}
		rowNumber++
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read PostgreSQL dump: %w", err)
	}
	if currentTable != "" {
		return fmt.Errorf("table %s COPY block is missing its terminator", currentTable)
	}
	if !sawCopy {
		return fmt.Errorf("backup SQL dump contains no COPY data blocks")
	}
	return nil
}

func parseCopyHeader(line string) (string, []string, error) {
	open := strings.IndexByte(line, '(')
	close := strings.LastIndexByte(line, ')')
	if open < 0 || close <= open || !strings.HasSuffix(line, " FROM stdin;") {
		return "", nil, fmt.Errorf("unsupported PostgreSQL COPY header: %s", line)
	}
	parts := strings.Fields(strings.TrimSpace(line[:open]))
	if len(parts) != 2 {
		return "", nil, fmt.Errorf("invalid PostgreSQL COPY table declaration: %s", line)
	}
	tableRef := strings.Split(parts[1], ".")
	if len(tableRef) != 2 || unquoteCopyIdentifier(tableRef[0]) != "public" {
		return "", nil, fmt.Errorf("unsupported PostgreSQL COPY schema: %s", parts[1])
	}
	table := unquoteCopyIdentifier(tableRef[1])
	if !restoreIdentifierPattern.MatchString(table) {
		return "", nil, fmt.Errorf("invalid PostgreSQL table identifier %q", table)
	}

	columnParts := strings.Split(line[open+1:close], ",")
	columns := make([]string, len(columnParts))
	for i, part := range columnParts {
		column := unquoteCopyIdentifier(strings.TrimSpace(part))
		if !restoreIdentifierPattern.MatchString(column) {
			return "", nil, fmt.Errorf("invalid PostgreSQL column identifier %q", column)
		}
		columns[i] = column
	}
	if len(columns) == 0 {
		return "", nil, fmt.Errorf("COPY table %s has no columns", table)
	}
	return table, columns, nil
}

func unquoteCopyIdentifier(identifier string) string {
	identifier = strings.TrimSpace(identifier)
	if len(identifier) >= 2 && identifier[0] == '"' && identifier[len(identifier)-1] == '"' {
		return strings.ReplaceAll(identifier[1:len(identifier)-1], `""`, `"`)
	}
	return identifier
}

func decodePostgresCopyField(value string) (interface{}, error) {
	if value == `\N` {
		return nil, nil
	}

	input := []byte(value)
	var output bytes.Buffer
	for i := 0; i < len(input); i++ {
		if input[i] != '\\' {
			output.WriteByte(input[i])
			continue
		}
		i++
		if i >= len(input) {
			return nil, fmt.Errorf("trailing escape character")
		}
		switch input[i] {
		case 'b':
			output.WriteByte('\b')
		case 'f':
			output.WriteByte('\f')
		case 'n':
			output.WriteByte('\n')
		case 'r':
			output.WriteByte('\r')
		case 't':
			output.WriteByte('\t')
		case 'v':
			output.WriteByte('\v')
		case '\\':
			output.WriteByte('\\')
		case '0', '1', '2', '3', '4', '5', '6', '7':
			end := i + 1
			for end < len(input) && end < i+3 && input[end] >= '0' && input[end] <= '7' {
				end++
			}
			parsed, err := strconv.ParseUint(string(input[i:end]), 8, 8)
			if err != nil {
				return nil, fmt.Errorf("invalid octal escape: %w", err)
			}
			output.WriteByte(byte(parsed))
			i = end - 1
		case 'x':
			end := i + 1
			for end < len(input) && end < i+3 && isHexDigit(input[end]) {
				end++
			}
			if end == i+1 {
				output.WriteByte('x')
				continue
			}
			parsed, err := strconv.ParseUint(string(input[i+1:end]), 16, 8)
			if err != nil {
				return nil, fmt.Errorf("invalid hexadecimal escape: %w", err)
			}
			output.WriteByte(byte(parsed))
			i = end - 1
		default:
			// PostgreSQL treats a backslash before a non-special character as
			// an escape for that character, which removes the backslash.
			output.WriteByte(input[i])
		}
	}
	return output.String(), nil
}

func isHexDigit(b byte) bool {
	return b >= '0' && b <= '9' || b >= 'a' && b <= 'f' || b >= 'A' && b <= 'F'
}
