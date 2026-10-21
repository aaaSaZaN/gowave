package database

import (
	"github.com/google/uuid"
	"os"
	"testing"
)

func TestMigrationsFreshAndExisting(t *testing.T) {
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
			sql, _ := db.DB()
			defer sql.Close()
			if dialect == "postgres" {
				name := "test_" + uuid.New().String()[:8]
				if err := db.Exec("CREATE SCHEMA " + name).Error; err != nil {
					t.Fatal(err)
				}
				defer db.Exec("DROP SCHEMA " + name + " CASCADE")
				sql.SetMaxOpenConns(1)
				if err := db.Exec("SET search_path TO " + name).Error; err != nil {
					t.Fatal(err)
				}
			}
			// Simulate upstream settings and a native UUID parent table.
			settingsDDL := `CREATE TABLE remnawave_settings (id integer PRIMARY KEY,passkey_settings text,oauth2_settings text,password_settings text,branding_settings text)`
			if err := db.Exec(settingsDDL).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Exec(`INSERT INTO remnawave_settings VALUES (1,'{}','{}','{"enabled":false}','{}')`).Error; err != nil {
				t.Fatal(err)
			}
			if dialect == "postgres" {
				if err := db.Exec(`CREATE TABLE admin (uuid uuid PRIMARY KEY,username varchar(64),password_hash varchar(255),role varchar(32),created_at timestamptz,updated_at timestamptz)`).Error; err != nil {
					t.Fatal(err)
				}
			}
			if err := AutoMigrate(db); err != nil {
				t.Fatal(err)
			}
			if err := AutoMigrate(db); err != nil {
				t.Fatal("migration replay:", err)
			}
			var settings RemnawaveSetting
			if err := db.First(&settings).Error; err != nil {
				t.Fatal(err)
			}
			if settings.PasswordSettings != `{"enabled":false}` || !settings.IsLoginAllowed {
				t.Fatalf("settings changed or not upgraded: %+v", settings)
			}
			if err := db.Create(&Admin{UUID: uuid.NewString(), Username: "test", PasswordHash: "preserved", Role: "ADMIN"}).Error; err != nil {
				t.Fatal(err)
			}
			var count int64
			db.Table("gowave_migrations").Count(&count)
			if count != 4 {
				t.Fatalf("expected 4 migrations, got %d", count)
			}
		})
	}
}

func TestFreshSchema(t *testing.T) {
	db, err := Init("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	sql, _ := db.DB()
	defer sql.Close()
	if err := AutoMigrate(db); err != nil {
		t.Fatal(err)
	}
	if len(SchemaTableOrder()) != len(schemaModels()) {
		t.Fatal("restore table list differs from model list")
	}
}
