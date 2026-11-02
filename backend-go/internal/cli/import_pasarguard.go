package cli

import (
	"archive/zip"
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"

	"gowave/internal/database"
	"gowave/internal/users"
)

type PasarGuardProxySettings struct {
	Vless struct {
		ID string `json:"id"`
	} `json:"vless"`
	Trojan struct {
		Password string `json:"password"`
	} `json:"trojan"`
	Shadowsocks struct {
		Password string `json:"password"`
		Method   string `json:"method"`
	} `json:"shadowsocks"`
	Hysteria struct {
		Auth string `json:"auth"`
	} `json:"hysteria"`
}

type PasarGuardRawUser struct {
	OnlineAt               *time.Time
	EditAt                 *time.Time
	SubRevokedAt           *time.Time
	ID                     uint64     `gorm:"column:id"`
	Username               string     `gorm:"column:username"`
	Status                 string     `gorm:"column:status"`
	UsedTraffic            uint64     `gorm:"column:used_traffic"`
	DataLimit              *uint64    `gorm:"column:data_limit"`
	DataLimitResetStrategy string     `gorm:"column:data_limit_reset_strategy"`
	Expire                 *time.Time `gorm:"column:expire"`
	CreatedAt              time.Time  `gorm:"column:created_at"`
	Note                   *string    `gorm:"column:note"`
	HWIDLimit              *int       `gorm:"column:hwid_limit"`
	ProxySettings          string     `gorm:"column:proxy_settings"`
}

type PasarGuardRawHWID struct {
	LastUsedAt  time.Time
	UserID      uint64    `gorm:"column:user_id"`
	HWID        string    `gorm:"column:hwid"`
	DeviceOS    *string   `gorm:"column:device_os"`
	OSVersion   *string   `gorm:"column:os_version"`
	DeviceModel *string   `gorm:"column:device_model"`
	CreatedAt   time.Time `gorm:"column:created_at"`
}

// pasarGuardEnvFiles hold SQLALCHEMY_DATABASE_URL; the PasarGuard installer
// writes /opt/pasarguard/.env.
var pasarGuardEnvFiles = []string{
	"/opt/pasarguard/.env",
	"/opt/pasarguard/data/.env",
	"./.env.pasarguard",
}

// pasarGuardSQLitePaths are tried when no .env names the database. The
// installer keeps SQLite in /var/lib/pasarguard, mounted at the same path in
// its container.
var pasarGuardSQLitePaths = []string{
	"/var/lib/pasarguard/db.sqlite3",
	"/opt/pasarguard/data/db.sqlite3",
	"/opt/pasarguard/db.sqlite3",
	"/etc/pasarguard/db.sqlite3",
	"./pasarguard.db",
	"./db.sqlite3",
}

func DetectPasarGuardSource(source string) (driver string, dsn string, err error) {
	if source != "" {
		if driver, dsn, ok, err := parsePasarGuardDatabaseURL(source, ""); err != nil || ok {
			return driver, dsn, err
		}
		if _, err := os.Stat(source); err == nil {
			return "sqlite", filepath.Clean(source), nil
		}
		return "", "", fmt.Errorf("specified source '%s' not found or invalid", source)
	}
	return detectPasarGuardSourceIn(pasarGuardEnvFiles, pasarGuardSQLitePaths)
}

func detectPasarGuardSourceIn(envFiles, sqlitePaths []string) (string, string, error) {
	// The .env names the database the panel actually uses; a SQLite file left
	// over from before a switch to PostgreSQL must not win over it.
	for _, envFile := range envFiles {
		data, err := os.ReadFile(envFile)
		if err != nil {
			continue
		}
		value, found := pasarGuardEnvValue(string(data), "SQLALCHEMY_DATABASE_URL")
		if !found {
			value, found = pasarGuardEnvValue(string(data), "DATABASE_URL")
		}
		if !found {
			continue
		}
		driver, dsn, ok, err := parsePasarGuardDatabaseURL(value, filepath.Dir(envFile))
		if err != nil {
			return "", "", err
		}
		if !ok {
			continue
		}
		if driver == "sqlite" {
			if _, err := os.Stat(dsn); err != nil {
				// Relative paths resolve inside the PasarGuard container.
				continue
			}
		}
		return driver, dsn, nil
	}

	for _, path := range sqlitePaths {
		if _, err := os.Stat(path); err == nil {
			return "sqlite", path, nil
		}
	}

	return "", "", fmt.Errorf("no PasarGuard database automatically detected. Please specify path or connection string manually")
}

// pasarGuardEnvValue returns the last uncommented assignment of key. The
// PasarGuard .env uses python-dotenv style `KEY = "value"` with spaces.
func pasarGuardEnvValue(data, key string) (string, bool) {
	value, found := "", false
	for _, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "export "))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		name, raw, ok := strings.Cut(line, "=")
		if !ok || strings.TrimSpace(name) != key {
			continue
		}
		raw = strings.TrimSpace(raw)
		if len(raw) >= 2 && (raw[0] == '"' || raw[0] == '\'') && raw[len(raw)-1] == raw[0] {
			raw = raw[1 : len(raw)-1]
		}
		value, found = raw, true
	}
	return value, found
}

// parsePasarGuardDatabaseURL converts an SQLAlchemy URL such as
// sqlite+aiosqlite:////var/lib/pasarguard/db.sqlite3 or
// postgresql+asyncpg://user:pass@127.0.0.1:6432/pasarguard into a GoWave
// driver and DSN. ok is false when raw is not a database URL.
func parsePasarGuardDatabaseURL(raw, baseDir string) (driver, dsn string, ok bool, err error) {
	scheme, rest, found := strings.Cut(strings.TrimSpace(raw), "://")
	if !found {
		return "", "", false, nil
	}
	dialect, _, _ := strings.Cut(strings.ToLower(scheme), "+")
	switch dialect {
	case "sqlite":
		// SQLAlchemy: sqlite:///relative.db and sqlite:////absolute.db
		path, _, _ := strings.Cut(strings.TrimPrefix(rest, "/"), "?")
		if path == "" {
			return "", "", false, fmt.Errorf("PasarGuard SQLite URL %q has no file path", raw)
		}
		if !filepath.IsAbs(path) && baseDir != "" {
			path = filepath.Join(baseDir, path)
		}
		// sqlite://///var/db.sqlite3 still names /var/db.sqlite3; the extra
		// slash must not reach the file: URI as an empty authority.
		return "sqlite", filepath.Clean(path), true, nil
	case "postgres", "postgresql", "timescaledb":
		parsed, err := url.Parse("postgresql://" + rest)
		if err != nil {
			return "", "", false, fmt.Errorf("invalid PasarGuard PostgreSQL URL: %w", err)
		}
		query := parsed.Query()
		// asyncpg's ssl option is not a libpq parameter; pgx would send it
		// to the server as a runtime setting.
		if ssl := strings.ToLower(query.Get("ssl")); ssl != "" {
			query.Del("ssl")
			modes := map[string]string{"true": "require", "false": "disable", "disable": "disable", "allow": "allow", "prefer": "prefer", "require": "require", "verify-ca": "verify-ca", "verify-full": "verify-full"}
			if mode, known := modes[ssl]; known && query.Get("sslmode") == "" {
				query.Set("sslmode", mode)
			}
		}
		// The installer connects through pgbouncer in transaction mode, where
		// prepared statements do not survive between queries.
		if query.Get("default_query_exec_mode") == "" {
			query.Set("default_query_exec_mode", "simple_protocol")
		}
		parsed.RawQuery = query.Encode()
		return "postgres", parsed.String(), true, nil
	case "mysql", "mariadb":
		return "", "", false, fmt.Errorf("PasarGuard uses %s; GoWave can import only from SQLite or PostgreSQL", dialect)
	default:
		return "", "", false, nil
	}
}

// sqliteReadOnlyURI builds a read-only file: URI for path. Concatenating
// "file:"+path breaks on //var/... (read as a URI authority, which SQLite
// reports as "out of memory") and on paths containing ?, # or %.
func sqliteReadOnlyURI(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	return (&url.URL{Scheme: "file", Path: filepath.ToSlash(filepath.Clean(path)), RawQuery: "mode=ro"}).String()
}

func ImportFromPasarGuard(destDB *gorm.DB, source string) error {
	if strings.HasSuffix(strings.ToLower(source), ".zip") {
		archive, err := zip.OpenReader(source)
		if err != nil {
			return err
		}
		defer archive.Close()
		for _, f := range archive.File {
			if f.Name != "db.sqlite3" {
				continue
			}
			r, err := f.Open()
			if err != nil {
				return err
			}
			defer r.Close()
			tmp, err := os.CreateTemp("", "gowave-pasar-*.sqlite3")
			if err != nil {
				return err
			}
			defer os.Remove(tmp.Name())
			_, copyErr := io.Copy(tmp, r)
			closeErr := tmp.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
			return ImportFromPasarGuard(destDB, tmp.Name())
		}
		return fmt.Errorf("backup has no root db.sqlite3")
	}
	driver, dsn, err := DetectPasarGuardSource(source)
	if err != nil {
		return err
	}

	fmt.Printf("[*] Opening PasarGuard database (%s)...\n", driver)

	gormCfg := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	}
	var srcDB *gorm.DB
	if driver == "sqlite" {
		// Read-only: PasarGuard may still be running on this file.
		srcDB, err = gorm.Open(sqlite.Open(sqliteReadOnlyURI(dsn)), gormCfg)
	} else {
		srcDB, err = gorm.Open(postgres.Open(dsn), gormCfg)
	}
	if err != nil {
		return fmt.Errorf("failed to connect to PasarGuard database: %w", err)
	}

	sqlDB, err := srcDB.DB()
	if err != nil {
		return err
	}
	defer sqlDB.Close()
	return destDB.Transaction(func(tx *gorm.DB) error { return importPasarGuard(tx, srcDB) })
}

func importPasarGuard(destDB, srcDB *gorm.DB) error {
	userIDs := map[uint64]uint64{}

	// 1. Read users
	var pgUsers []PasarGuardRawUser
	err := srcDB.Table("users").Find(&pgUsers).Error
	if err != nil {
		return fmt.Errorf("failed to query users table: %w", err)
	}

	fmt.Printf("[+] Found %d users in PasarGuard database.\n", len(pgUsers))

	// Find default internal squad if exists
	var defaultSquadUUID string
	var squad database.InternalSquad
	if err := destDB.First(&squad).Error; err == nil && squad.UUID != "" && !srcDB.Migrator().HasTable("users_groups_association") {
		defaultSquadUUID = squad.UUID
		fmt.Printf("[*] Found default internal squad '%s' (%s). Users will be assigned to it.\n", squad.Name, squad.UUID)
	}

	farFuture := time.Date(2099, 10, 8, 0, 0, 0, 0, time.UTC)
	importedCount := 0
	now := time.Now().UTC()

	for _, u := range pgUsers {
		var ps PasarGuardProxySettings
		if u.ProxySettings != "" {
			if err := json.Unmarshal([]byte(u.ProxySettings), &ps); err != nil {
				return fmt.Errorf("user %d proxy settings: %w", u.ID, err)
			}
		}

		vlessUUID := strings.TrimSpace(ps.Vless.ID)
		if vlessUUID == "" {
			vlessUUID = uuid.NewString()
		}
		trojanPassword := strings.TrimSpace(ps.Trojan.Password)
		if trojanPassword == "" {
			trojanPassword = users.NewPassword()
		}
		ssPassword := strings.TrimSpace(ps.Shadowsocks.Password)
		if ssPassword == "" {
			ssPassword = users.NewPassword()
		}

		status := strings.ToUpper(strings.TrimSpace(u.Status))
		switch status {
		case "ON_HOLD":
			status = "DISABLED"
		case "ACTIVE":
			status = "ACTIVE"
		case "DISABLED":
			status = "DISABLED"
		case "LIMITED":
			status = "LIMITED"
		case "EXPIRED":
			status = "EXPIRED"
		default:
			status = "ACTIVE"
		}

		strategy := strings.ToUpper(strings.TrimSpace(u.DataLimitResetStrategy))
		switch strategy {
		case "DAY":
			strategy = "DAY"
		case "WEEK":
			strategy = "WEEK"
		case "MONTH":
			strategy = "MONTH"
		case "YEAR":
			strategy = "YEAR"
		default:
			strategy = "NO_RESET"
		}

		expireAt := farFuture
		if u.Expire != nil && u.Expire.Year() > 2000 {
			expireAt = u.Expire.UTC()
		}

		var limitBytes uint64 = 0
		if u.DataLimit != nil {
			limitBytes = *u.DataLimit
		}

		noteStr := ""
		if u.Note != nil {
			noteStr = *u.Note
		}

		gwUser := database.User{
			SubRevokedAt:         u.SubRevokedAt,
			ShortUUID:            users.NewShortUUID(),
			Username:             u.Username,
			Status:               status,
			TrafficLimitBytes:    limitBytes,
			TrafficLimitStrategy: strategy,
			ExpireAt:             expireAt,
			TrojanPassword:       trojanPassword,
			VlessUUID:            vlessUUID,
			SsPassword:           ssPassword,
			Description:          noteStr,
			HWIDDeviceLimit:      u.HWIDLimit,
			CreatedAt:            u.CreatedAt.UTC(),
			UpdatedAt:            now,
		}

		// Insert or update in Gowave
		err := destDB.Transaction(func(tx *gorm.DB) error {
			var existing database.User
			if err := tx.Where("username = ?", u.Username).First(&existing).Error; err == nil {
				gwUser.ID = existing.ID
				gwUser.ShortUUID = existing.ShortUUID
				if ps.Vless.ID == "" {
					gwUser.VlessUUID = existing.VlessUUID
				}
				if ps.Trojan.Password == "" {
					gwUser.TrojanPassword = existing.TrojanPassword
				}
				if ps.Shadowsocks.Password == "" {
					gwUser.SsPassword = existing.SsPassword
				}
				if err := tx.Model(&database.User{}).Where("id = ?", existing.ID).Select("*").Omit("Traffic", "ExternalSquad", "ActiveInternalSquads").Updates(gwUser).Error; err != nil {
					return err
				}
			} else if err != gorm.ErrRecordNotFound {
				return err
			} else {
				if err := tx.Create(&gwUser).Error; err != nil {
					return err
				}
			}

			userIDs[u.ID] = gwUser.ID
			traffic := database.UserTraffic{
				ID:                       gwUser.ID,
				UsedTrafficBytes:         u.UsedTraffic,
				LifetimeUsedTrafficBytes: u.UsedTraffic,
				OnlineAt:                 u.OnlineAt,
			}
			if err := tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(&traffic).Error; err != nil {
				return err
			}

			if defaultSquadUUID != "" {
				member := database.InternalSquadMember{
					InternalSquadUUID: defaultSquadUUID,
					UserID:            gwUser.ID,
				}
				if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&member).Error; err != nil {
					return err
				}
			}
			return nil
		})

		if err != nil {
			return fmt.Errorf("import user %d: %w", u.ID, err)
		} else {
			importedCount++
		}
	}

	fmt.Printf("[+] Successfully imported/updated %d users!\n", importedCount)

	// 2. Read HWIDs if table exists
	var pgHwids []PasarGuardRawHWID
	if srcDB.Migrator().HasTable("user_hwids") {
		if err := srcDB.Table("user_hwids").Find(&pgHwids).Error; err != nil {
			return err
		}
		hwidCount := 0
		for _, h := range pgHwids {
			d := database.HwidDevice{
				HWID:        h.HWID,
				UserID:      userIDs[h.UserID],
				Platform:    h.DeviceOS,
				OSVersion:   h.OSVersion,
				DeviceModel: h.DeviceModel,
				CreatedAt:   h.CreatedAt.UTC(),
				UpdatedAt:   h.LastUsedAt,
			}
			if err := destDB.Clauses(clause.OnConflict{UpdateAll: true}).Create(&d).Error; err == nil {
				hwidCount++
			} else {
				return err
			}
		}
		fmt.Printf("[+] Successfully imported %d HWID devices!\n", hwidCount)
	}

	if err := importPasarGuardExtended(destDB, srcDB, userIDs); err != nil {
		return err
	}
	fmt.Println("\n[✔] Migration from PasarGuard completed successfully!")
	return nil
}

func ImportPasarGuardInteractive(db *gorm.DB, reader *bufio.Reader) {
	fmt.Println("\n--- Import Database from PasarGuard ---")
	fmt.Print("Enter PasarGuard database path, postgres URL, or press ENTER for auto-detect: ")
	input, _ := reader.ReadString('\n')
	src := strings.TrimSpace(input)

	err := ImportFromPasarGuard(db, src)
	if err != nil {
		fmt.Printf("[-] Import error: %v\n", err)
	}
}
