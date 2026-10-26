package database

import (
	"embed"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"gowave/internal/responserules"

	"gorm.io/gorm"
)

//go:embed migrations/*/*.sql
var migrationFiles embed.FS

// applyMigrations uses reviewed, numbered SQL. Existing upstream column types
// and data remain intact; GORM's destructive type reconciliation is not used.
func applyMigrations(db *gorm.DB) error {
	dialect := db.Dialector.Name()
	if dialect != "sqlite" && dialect != "postgres" {
		return fmt.Errorf("unsupported dialect %s", dialect)
	}
	entries, err := migrationFiles.ReadDir("migrations/" + dialect)
	if err != nil {
		return err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	return db.Transaction(func(tx *gorm.DB) error {
		if dialect == "postgres" {
			if err := tx.Exec("SELECT pg_advisory_xact_lock(747339274)").Error; err != nil {
				return err
			}
		}
		if err := tx.Exec(`CREATE TABLE IF NOT EXISTS gowave_migrations (name varchar(255) PRIMARY KEY, applied_at timestamp NOT NULL)`).Error; err != nil {
			return err
		}
		for _, entry := range entries {
			var count int64
			if err := tx.Table("gowave_migrations").Where("name = ?", entry.Name()).Count(&count).Error; err != nil {
				return err
			}
			if count != 0 {
				continue
			}
			data, err := migrationFiles.ReadFile("migrations/" + dialect + "/" + entry.Name())
			if err != nil {
				return err
			}
			for _, line := range strings.Split(string(data), "\n") {
				line = strings.TrimSpace(line)
				if line == "" || strings.HasPrefix(line, "--") {
					continue
				}
				if strings.HasPrefix(line, "ADD COLUMN ") {
					parts := strings.SplitN(strings.TrimPrefix(line, "ADD COLUMN "), " ", 3)
					if len(parts) != 3 {
						return fmt.Errorf("invalid column migration: %s", line)
					}
					if !tx.Migrator().HasColumn(parts[0], parts[1]) {
						query := fmt.Sprintf(`ALTER TABLE "%s" ADD COLUMN "%s" %s`, parts[0], parts[1], parts[2])
						if err := tx.Exec(query).Error; err != nil {
							return fmt.Errorf("%s: %w", entry.Name(), err)
						}
					}
					continue
				}
				if dialect == "postgres" && strings.HasPrefix(line, "CREATE TABLE ") {
					line, err = matchImportedForeignKeyTypes(tx, line)
					if err != nil {
						return err
					}
				}
				if err := tx.Exec(line).Error; err != nil {
					return fmt.Errorf("migration %s: %w", entry.Name(), err)
				}
			}
			if entry.Name() == "006_response_rule_headers.sql" {
				if err := migrateResponseRuleHeaders(tx); err != nil {
					return fmt.Errorf("migration %s: %w", entry.Name(), err)
				}
			}
			if err := tx.Exec("INSERT INTO gowave_migrations (name, applied_at) VALUES (?, ?)", entry.Name(), time.Now().UTC()).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func migrateResponseRuleHeaders(tx *gorm.DB) error {
	var settings []SubscriptionSetting
	if err := tx.Select("uuid", "response_rules").Find(&settings).Error; err != nil {
		return err
	}
	for _, setting := range settings {
		if strings.TrimSpace(setting.ResponseRules) == "" {
			continue
		}
		normalized, err := responserules.Normalize([]byte(setting.ResponseRules))
		if err != nil {
			return fmt.Errorf("subscription settings %s: %w", setting.UUID, err)
		}
		if string(normalized) != setting.ResponseRules {
			if err := tx.Model(&SubscriptionSetting{}).Where("uuid = ?", setting.UUID).
				UpdateColumn("response_rules", string(normalized)).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

var foreignKeyPattern = regexp.MustCompile(`FOREIGN KEY \("([a-z_]+)"\) REFERENCES "([a-z_]+)"\("([a-z_]+)"\)`)

// SchemaTableOrder is a stable parent-before-child order for data restores.
func SchemaTableOrder() []string {
	data, _ := migrationFiles.ReadFile("migrations/sqlite/001_initial.sql")
	pattern := regexp.MustCompile("CREATE TABLE IF NOT EXISTS `([a-z_]+)`")
	var result []string
	for _, match := range pattern.FindAllStringSubmatch(string(data), -1) {
		result = append(result, match[1])
	}
	return result
}

// Upstream stores UUIDs as native PostgreSQL uuid, while a fresh Go schema uses
// varchar. Newly created relations must use the existing parent's exact type.
func matchImportedForeignKeyTypes(tx *gorm.DB, ddl string) (string, error) {
	for _, match := range foreignKeyPattern.FindAllStringSubmatch(ddl, -1) {
		var typ string
		if err := tx.Raw(`SELECT udt_name FROM information_schema.columns WHERE table_schema=current_schema() AND table_name=? AND column_name=?`, match[2], match[3]).Scan(&typ).Error; err != nil {
			return "", err
		}
		if typ == "uuid" {
			ddl = strings.Replace(ddl, `"`+match[1]+`" varchar(64)`, `"`+match[1]+`" uuid`, 1)
		}
	}
	return ddl, nil
}
