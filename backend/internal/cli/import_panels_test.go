package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gowave/internal/database"
	"gowave/internal/testdb"
)

func sourceFixture(t *testing.T, statements ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "source.db")
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range statements {
		if err := db.Exec(s).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}
func TestPasarGuardImportRemapsAndReimports(t *testing.T) {
	src := sourceFixture(t,
		`CREATE TABLE users (id INTEGER,username TEXT,status TEXT,used_traffic INTEGER,data_limit INTEGER,proxy_settings TEXT)`,
		`INSERT INTO users VALUES(1,'imported','disabled',123,0,'{"vless":{"id":"a4dd626c-d4ac-4833-97b8-ff4b9caadbef"}}')`,
		`CREATE TABLE user_hwids(user_id INTEGER,hwid TEXT)`,
		`INSERT INTO user_hwids VALUES(1,'shared')`)
	dst := testdb.Open(t)
	unrelated := database.User{Username: "unrelated", ShortUUID: "unrelated"}
	if err := dst.Create(&unrelated).Error; err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := ImportFromPasarGuard(dst, src); err != nil {
			t.Fatal(err)
		}
	}
	var u database.User
	if err := dst.First(&u, "username = ?", "imported").Error; err != nil {
		t.Fatal(err)
	}
	if u.ID == unrelated.ID || u.Status != "DISABLED" || u.TrafficLimitBytes != 0 {
		t.Fatalf("bad mapped user: id=%d status=%s", u.ID, u.Status)
	}
	var d database.HwidDevice
	if err := dst.First(&d, "hwid = ?", "shared").Error; err != nil {
		t.Fatal(err)
	}
	if d.UserID != u.ID {
		t.Fatal("device points at unrelated user")
	}
	var traffic database.UserTraffic
	if err := dst.First(&traffic, "id = ?", u.ID).Error; err != nil {
		t.Fatal(err)
	}
	if traffic.UsedTrafficBytes != 123 {
		t.Fatal("traffic not imported")
	}
}
func Test3xUIImportAndRollback(t *testing.T) {
	src := sourceFixture(t,
		`CREATE TABLE inbounds (id INTEGER,protocol TEXT,settings TEXT,stream_settings TEXT,tag TEXT,port INTEGER,enable BOOLEAN)`,
		`INSERT INTO inbounds VALUES(1,'vless','{"clients":[{"id":"a4dd626c-d4ac-4833-97b8-ff4b9caadbef","email":"client","enable":true,"totalGB":1000,"subId":"old-link","tgId":"1234"},{"id":"8cc5f80f-6e26-4e60-b4b9-81f8b018a326","email":"second","enable":true,"subId":"old-link","tgId":""}]}','{"network":"tcp","security":"reality"}','test',443,1)`,
		`CREATE TABLE client_traffics(inbound_id INTEGER,email TEXT,up INTEGER,down INTEGER,all_time INTEGER,last_online INTEGER)`,
		`INSERT INTO client_traffics VALUES(1,'client',10,20,45,0)`)
	dst := testdb.Open(t)
	for range 2 {
		if err := ImportFrom3xUI(dst, src); err != nil {
			t.Fatal(err)
		}
	}
	var u database.User
	if err := dst.First(&u, "username = ?", "client").Error; err != nil {
		t.Fatal(err)
	}
	if u.ShortUUID != "old-link" || u.TrafficLimitBytes != 1000 || u.TelegramID == nil || *u.TelegramID != 1234 {
		t.Fatal("client fields lost")
	}
	var traffic database.UserTraffic
	if err := dst.First(&traffic, "id = ?", u.ID).Error; err != nil {
		t.Fatal(err)
	}
	if traffic.UsedTrafficBytes != 30 || traffic.LifetimeUsedTrafficBytes != 45 {
		t.Fatal("traffic lost")
	}
	bad := sourceFixture(t, `CREATE TABLE inbounds(id INTEGER,protocol TEXT,settings TEXT,tag TEXT)`, `INSERT INTO inbounds VALUES(2,'vmess','{}','unsupported')`)
	if err := ImportFrom3xUI(dst, bad); err == nil {
		t.Fatal("unsupported protocol must fail")
	}
	var second database.User
	if err := dst.First(&second, "username = ?", "second").Error; err != nil {
		t.Fatal(err)
	}
	if second.ShortUUID == "old-link" || second.TelegramID != nil {
		t.Fatal("shared subscription or empty telegram ID incorrectly mapped")
	}
	var count int64
	dst.Model(&database.User{}).Count(&count)
	if count != 2 {
		t.Fatal("unexpected writes after failed import")
	}
}

// Opt-in local verification; production data never becomes a fixture.
func TestPasarGuardProductionBackup(t *testing.T) {
	source := os.Getenv("GOWAVE_TEST_PASARGUARD_BACKUP")
	if source == "" {
		t.Skip("set GOWAVE_TEST_PASARGUARD_BACKUP to verify a private backup")
	}
	dst := testdb.Open(t)
	for range 2 {
		if err := ImportFromPasarGuard(dst, source); err != nil {
			t.Fatal(err)
		}
	}
	for _, table := range []string{"users", "hwid_user_devices", "nodes", "hosts", "config_profiles", "nodes_usage_history", "nodes_user_usage_history", "user_subscription_request_history"} {
		var count int64
		if err := dst.Table(table).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		t.Logf("%s: %d", table, count)
	}
}

func TestPasarGuardImportRollback(t *testing.T) {
	src := sourceFixture(t,
		`CREATE TABLE users(id INTEGER,username TEXT,proxy_settings TEXT)`,
		`INSERT INTO users VALUES(1,'valid','{}'),(2,'invalid','broken')`)
	dst := testdb.Open(t)
	if err := ImportFromPasarGuard(dst, src); err == nil {
		t.Fatal("malformed settings accepted")
	}
	var count int64
	if err := dst.Model(&database.User{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("failed import retained earlier users")
	}
}
