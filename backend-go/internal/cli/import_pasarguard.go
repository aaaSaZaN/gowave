package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"remnawave-go/internal/database"
	"remnawave-go/internal/users"
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
	UserID      uint64    `gorm:"column:user_id"`
	HWID        string    `gorm:"column:hwid"`
	DeviceOS    *string   `gorm:"column:device_os"`
	OSVersion   *string   `gorm:"column:os_version"`
	DeviceModel *string   `gorm:"column:device_model"`
	CreatedAt   time.Time `gorm:"column:created_at"`
}

func DetectPasarGuardSource(source string) (driver string, dsn string, err error) {
	if source != "" {
		if strings.HasPrefix(source, "postgres://") || strings.HasPrefix(source, "postgresql://") {
			return "postgres", source, nil
		}
		if _, err := os.Stat(source); err == nil {
			return "sqlite", source, nil
		}
		return "", "", fmt.Errorf("specified source '%s' not found or invalid", source)
	}

	// 1. Common SQLite paths
	candidates := []string{
		"/opt/pasarguard/data/db.sqlite3",
		"/opt/pasarguard/db.sqlite3",
		"/var/lib/pasarguard/db.sqlite3",
		"/etc/pasarguard/db.sqlite3",
		"./pasarguard.db",
		"./db.sqlite3",
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return "sqlite", p, nil
		}
	}

	// 2. Check /opt/pasarguard/.env for database URL
	envFiles := []string{
		"/opt/pasarguard/.env",
		"/opt/pasarguard/data/.env",
		"./.env.pasarguard",
	}
	for _, ef := range envFiles {
		if data, err := os.ReadFile(ef); err == nil {
			lines := strings.Split(string(data), "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "SQLALCHEMY_DATABASE_URL=") || strings.HasPrefix(line, "DATABASE_URL=") {
					parts := strings.SplitN(line, "=", 2)
					if len(parts) == 2 {
						val := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
						if strings.HasPrefix(val, "sqlite:////") {
							p := strings.TrimPrefix(val, "sqlite:////")
							p = "/" + strings.TrimLeft(p, "/")
							if _, err := os.Stat(p); err == nil {
								return "sqlite", p, nil
							}
						} else if strings.HasPrefix(val, "sqlite:///") {
							p := strings.TrimPrefix(val, "sqlite:///")
							if !filepath.IsAbs(p) {
								p = filepath.Join(filepath.Dir(ef), p)
							}
							if _, err := os.Stat(p); err == nil {
								return "sqlite", p, nil
							}
						} else if strings.HasPrefix(val, "postgres://") || strings.HasPrefix(val, "postgresql://") {
							return "postgres", val, nil
						}
					}
				}
			}
		}
	}

	return "", "", fmt.Errorf("no PasarGuard database automatically detected. Please specify path or connection string manually")
}

func ImportFromPasarGuard(destDB *gorm.DB, source string) error {
	driver, dsn, err := DetectPasarGuardSource(source)
	if err != nil {
		return err
	}

	fmt.Printf("[*] Opening PasarGuard database (%s: %s)...\n", driver, dsn)

	gormCfg := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	}
	var srcDB *gorm.DB
	if driver == "sqlite" {
		srcDB, err = gorm.Open(sqlite.Open(dsn), gormCfg)
	} else {
		srcDB, err = gorm.Open(postgres.Open(dsn), gormCfg)
	}
	if err != nil {
		return fmt.Errorf("failed to connect to PasarGuard database: %w", err)
	}

	// 1. Read users
	var pgUsers []PasarGuardRawUser
	err = srcDB.Table("users").Find(&pgUsers).Error
	if err != nil {
		return fmt.Errorf("failed to query users table: %w", err)
	}

	fmt.Printf("[+] Found %d users in PasarGuard database.\n", len(pgUsers))

	// Find default internal squad if exists
	var defaultSquadUUID string
	var squad database.InternalSquad
	if err := destDB.First(&squad).Error; err == nil && squad.UUID != "" {
		defaultSquadUUID = squad.UUID
		fmt.Printf("[*] Found default internal squad '%s' (%s). Users will be assigned to it.\n", squad.Name, squad.UUID)
	}

	farFuture := time.Date(2099, 10, 8, 0, 0, 0, 0, time.UTC)
	importedCount := 0
	now := time.Now().UTC()

	for _, u := range pgUsers {
		var ps PasarGuardProxySettings
		if u.ProxySettings != "" {
			_ = json.Unmarshal([]byte(u.ProxySettings), &ps)
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
		case "ACTIVE", "ON_HOLD":
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
			ID:                   u.ID,
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
			if err := tx.Where("id = ? OR username = ?", u.ID, u.Username).First(&existing).Error; err == nil {
				gwUser.ID = existing.ID
				gwUser.ShortUUID = existing.ShortUUID
				return tx.Model(&database.User{}).Where("id = ?", existing.ID).Updates(gwUser).Error
			}

			if err := tx.Create(&gwUser).Error; err != nil {
				return err
			}

			traffic := database.UserTraffic{
				ID:                       gwUser.ID,
				UsedTrafficBytes:         u.UsedTraffic,
				LifetimeUsedTrafficBytes: u.UsedTraffic,
			}
			_ = tx.Create(&traffic).Error

			if defaultSquadUUID != "" {
				member := database.InternalSquadMember{
					InternalSquadUUID: defaultSquadUUID,
					UserID:            gwUser.ID,
				}
				_ = tx.Create(&member).Error
			}
			return nil
		})

		if err != nil {
			fmt.Printf("[-] Error importing user %s: %v\n", u.Username, err)
		} else {
			importedCount++
		}
	}

	fmt.Printf("[+] Successfully imported/updated %d users!\n", importedCount)

	// 2. Read HWIDs if table exists
	var pgHwids []PasarGuardRawHWID
	if err := srcDB.Table("user_hwids").Find(&pgHwids).Error; err == nil && len(pgHwids) > 0 {
		hwidCount := 0
		for _, h := range pgHwids {
			d := database.HwidDevice{
				HWID:        h.HWID,
				UserID:      h.UserID,
				Platform:    h.DeviceOS,
				OSVersion:   h.OSVersion,
				DeviceModel: h.DeviceModel,
				CreatedAt:   h.CreatedAt.UTC(),
				UpdatedAt:   now,
			}
			if err := destDB.Where("hwid = ?", h.HWID).FirstOrCreate(&d).Error; err == nil {
				hwidCount++
			}
		}
		fmt.Printf("[+] Successfully imported %d HWID devices!\n", hwidCount)
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
