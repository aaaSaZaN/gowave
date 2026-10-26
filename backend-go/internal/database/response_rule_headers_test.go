package database

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestResponseRuleHeadersUpgrade(t *testing.T) {
	for _, dialect := range []string{"sqlite", "postgres"} {
		t.Run(dialect, func(t *testing.T) {
			dsn := ":memory:"
			if dialect == "postgres" {
				dsn = os.Getenv("GOWAVE_TEST_POSTGRES")
				if dsn == "" {
					t.Skip("GOWAVE_TEST_POSTGRES not set")
				}
			}
			db, err := Init(dialect, dsn)
			if err != nil {
				t.Fatal(err)
			}
			conn, _ := db.DB()
			defer conn.Close()
			if dialect == "postgres" {
				name := "headers_" + uuid.NewString()[:8]
				if err := db.Exec("CREATE SCHEMA " + name).Error; err != nil {
					t.Fatal(err)
				}
				defer db.Exec("DROP SCHEMA " + name + " CASCADE")
				conn.SetMaxOpenConns(1)
				if err := db.Exec("SET search_path TO " + name).Error; err != nil {
					t.Fatal(err)
				}
			}
			if err := MigrateSchema(db); err != nil {
				t.Fatal(err)
			}
			legacy := `{"version":"1","future":9007199254740993,"rules":[{"name":"preserved","responseModifications":{"headers":[{"key":"X-Test","value":"first"},{"key":"x-test","value":"last"}],"future":true}},{"responseModifications":{"headers":[]}},{}]}`
			canonical := ` {"version":"1","rules":[{"responseModifications":{"headers":{"x-custom":"value"}}}]} `
			stamp := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
			rows := []SubscriptionSetting{
				{UUID: uuid.NewString(), ResponseRules: legacy, UpdatedAt: stamp},
				{UUID: uuid.NewString(), ResponseRules: canonical, UpdatedAt: stamp},
				{UUID: uuid.NewString(), ResponseRules: "null", UpdatedAt: stamp},
			}
			for i := range rows {
				if err := db.Create(&rows[i]).Error; err != nil {
					t.Fatal(err)
				}
			}
			if err := db.Exec("DELETE FROM gowave_migrations WHERE name = ?", "006_response_rule_headers.sql").Error; err != nil {
				t.Fatal(err)
			}
			// A bad row must roll back both conversion and the migration marker.
			invalid := SubscriptionSetting{UUID: uuid.NewString(), ResponseRules: `{"rules":[{"responseModifications":{"headers":[{}]}}]}`}
			if err := db.Create(&invalid).Error; err != nil {
				t.Fatal(err)
			}
			if err := MigrateSchema(db); err == nil {
				t.Fatal("invalid legacy headers did not stop migration")
			}
			var untouched SubscriptionSetting
			if err := db.First(&untouched, "uuid = ?", rows[0].UUID).Error; err != nil {
				t.Fatal(err)
			}
			if untouched.ResponseRules != legacy {
				t.Fatal("failed migration changed stored rules")
			}
			var count int64
			if err := db.Table("gowave_migrations").Where("name = ?", "006_response_rule_headers.sql").Count(&count).Error; err != nil || count != 0 {
				t.Fatalf("failed migration was marked applied: count=%d error=%v", count, err)
			}
			if err := db.Delete(&invalid).Error; err != nil {
				t.Fatal(err)
			}
			if err := MigrateSchema(db); err != nil {
				t.Fatal(err)
			}
			for i, row := range rows {
				var saved SubscriptionSetting
				if err := db.First(&saved, "uuid = ?", row.UUID).Error; err != nil {
					t.Fatal(err)
				}
				if !saved.UpdatedAt.Equal(stamp) {
					t.Fatal("migration changed the settings timestamp")
				}
				if i == 0 {
					for _, want := range []string{`"headers":{"x-test":"last"}`, `"headers":{}`, `9007199254740993`, `"future":true`, `"name":"preserved"`} {
						if !strings.Contains(saved.ResponseRules, want) {
							t.Fatalf("lost %s: %s", want, saved.ResponseRules)
						}
					}
				} else if saved.ResponseRules != row.ResponseRules {
					t.Fatalf("non-legacy row changed: %s", saved.ResponseRules)
				}
				rows[i] = saved
			}
			if err := MigrateSchema(db); err != nil {
				t.Fatal("migration replay:", err)
			}
			var replayed SubscriptionSetting
			if err := db.First(&replayed, "uuid = ?", rows[0].UUID).Error; err != nil || replayed.ResponseRules != rows[0].ResponseRules {
				t.Fatalf("migration replay changed rules: %+v (%v)", replayed, err)
			}
		})
	}
}
