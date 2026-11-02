package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gowave/internal/database"
	"gowave/internal/users"
)

// Schema reference: MHSanaei/3x-ui v2.8.5 database/model/model.go.
func ImportFrom3xUI(dst *gorm.DB, source string) error {
	if source == "" {
		source = "/etc/x-ui/x-ui.db"
	}
	if _, err := os.Stat(source); err != nil {
		return err
	}
	src, err := gorm.Open(sqlite.Open(sqliteReadOnlyURI(source)), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return err
	}
	sqlDB, err := src.DB()
	if err != nil {
		return err
	}
	defer sqlDB.Close()
	var inbounds []struct {
		ID                                                                uint64
		Protocol, Settings, StreamSettings, Sniffing, Tag, Listen, Remark string
		Port                                                              int
		Enable                                                            bool
	}
	if err := src.Table("inbounds").Order("id").Find(&inbounds).Error; err != nil {
		return err
	}
	return dst.Transaction(func(tx *gorm.DB) error {
		for _, b := range inbounds {
			if b.Protocol != "vless" && b.Protocol != "trojan" && b.Protocol != "shadowsocks" {
				return fmt.Errorf("inbound %d protocol %s cannot be represented by GoWave users", b.ID, b.Protocol)
			}
			var settings struct {
				Clients []struct {
					ID, Password, Email, SubID, Comment string
					TotalGB                             uint64
					ExpiryTime                          int64
					Enable                              bool
					TgID                                json.RawMessage `json:"tgId"`
					CreatedAt                           int64           `json:"created_at"`
					UpdatedAt                           int64           `json:"updated_at"`
					Reset                               int
				}
			}
			if err := json.Unmarshal([]byte(b.Settings), &settings); err != nil {
				return fmt.Errorf("inbound %d settings: %w", b.ID, err)
			}
			raw := map[string]interface{}{"tag": b.Tag, "port": b.Port, "protocol": b.Protocol, "listen": b.Listen, "settings": json.RawMessage(b.Settings)}
			if b.StreamSettings != "" {
				raw["streamSettings"] = json.RawMessage(b.StreamSettings)
			}
			if b.Sniffing != "" {
				raw["sniffing"] = json.RawMessage(b.Sniffing)
			}
			config, err := json.Marshal(map[string]interface{}{"inbounds": []interface{}{raw}, "outbounds": []interface{}{map[string]string{"protocol": "freedom", "tag": "direct"}}})
			if err != nil {
				return err
			}
			tags, err := importProfile(tx, "3x-ui", b.ID, b.Remark, string(config))
			if err != nil {
				return err
			}
			squad := database.InternalSquad{UUID: importUUID("3x-ui", "inbound", b.ID), Name: fmt.Sprintf("3x-ui-%d-%s", b.ID, b.Tag)}
			if err := importSave(tx, &squad); err != nil {
				return err
			}
			if err := importSave(tx, &database.InternalSquadInbound{InternalSquadUUID: squad.UUID, InboundUUID: tags[b.Tag]}); err != nil {
				return err
			}
			for i, c := range settings.Clients {
				if c.ExpiryTime < 0 {
					return fmt.Errorf("inbound %d client %d has deferred expiry; activate or convert it before import", b.ID, i)
				}
				// Keep clients separate: equal emails/subIds across inbounds do not imply
				// equal credentials, limits or counters.
				name := fmt.Sprintf("3xui_%d_%d", b.ID, i)
				if c.Email != "" {
					name = c.Email
				}
				var existing database.User
				lookup := tx.Where("username = ?", name).First(&existing).Error
				if lookup != nil && lookup != gorm.ErrRecordNotFound {
					return lookup
				}
				u := database.User{Username: name, ShortUUID: users.NewShortUUID(), VlessUUID: uuid.NewString(), TrojanPassword: users.NewPassword(), SsPassword: users.NewPassword(), Status: "ACTIVE", TrafficLimitBytes: c.TotalGB, TrafficLimitStrategy: "NO_RESET", ExpireAt: time.Date(2099, 10, 8, 0, 0, 0, 0, time.UTC), Description: c.Comment, Email: c.Email}
				if !c.Enable || !b.Enable {
					u.Status = "DISABLED"
				}
				if c.ExpiryTime > 0 {
					u.ExpireAt = time.UnixMilli(c.ExpiryTime).UTC()
					if u.Status == "ACTIVE" && u.ExpireAt.Before(time.Now()) {
						u.Status = "EXPIRED"
					}
				}
				if c.CreatedAt > 0 {
					u.CreatedAt = time.UnixMilli(c.CreatedAt).UTC()
				}
				if c.SubID != "" {
					u.ShortUUID = c.SubID
					var used int64
					if err := tx.Model(&database.User{}).Where("short_uuid = ? AND username <> ?", c.SubID, name).Count(&used).Error; err != nil {
						return err
					}
					if used > 0 {
						u.ShortUUID = uuid.NewSHA1(uuid.NameSpaceURL, []byte(fmt.Sprintf("gowave/3x-ui/%d/%s", b.ID, name))).String()
						fmt.Printf("[*] Client %q shares a subId; assigned a separate subscription token.\n", name)
					}
				}
				if tg := strings.Trim(string(c.TgID), "\""); tg != "" && tg != "null" && tg != "0" {
					id, err := strconv.ParseInt(tg, 10, 64)
					if err != nil {
						return err
					}
					u.TelegramID = &id
				}
				switch c.Reset {
				case 1:
					u.TrafficLimitStrategy = "DAY"
				case 7:
					u.TrafficLimitStrategy = "WEEK"
				case 30:
					u.TrafficLimitStrategy = "MONTH"
				case 365:
					u.TrafficLimitStrategy = "YEAR"
				}
				switch b.Protocol {
				case "vless":
					if _, err := uuid.Parse(c.ID); err != nil {
						return fmt.Errorf("invalid client UUID in inbound %d", b.ID)
					}
					u.VlessUUID = c.ID
				case "trojan":
					u.TrojanPassword = c.Password
				case "shadowsocks":
					u.SsPassword = c.Password
				}
				if lookup == nil {
					// Never merge a same-name account with different protocol credentials.
					if (b.Protocol == "vless" && existing.VlessUUID != c.ID) || (b.Protocol == "trojan" && existing.TrojanPassword != c.Password) || (b.Protocol == "shadowsocks" && existing.SsPassword != c.Password) {
						return fmt.Errorf("client %q has conflicting credentials; use distinct emails before importing", name)
					}
					u.ID = existing.ID
					u.ShortUUID = existing.ShortUUID
				}
				if err := importSave(tx, &u); err != nil {
					return err
				}
				traffic := database.UserTraffic{ID: u.ID}
				if src.Migrator().HasTable("client_traffics") {
					var stat struct {
						Up, Down, AllTime uint64
						LastOnline        int64
					}
					if err := src.Table("client_traffics").Where("inbound_id = ? AND email = ?", b.ID, c.Email).Find(&stat).Error; err != nil {
						return err
					}
					traffic.UsedTrafficBytes = stat.Up + stat.Down
					traffic.LifetimeUsedTrafficBytes = stat.AllTime
					if traffic.LifetimeUsedTrafficBytes < traffic.UsedTrafficBytes {
						traffic.LifetimeUsedTrafficBytes = traffic.UsedTrafficBytes
					}
					if stat.LastOnline > 0 {
						t := time.UnixMilli(stat.LastOnline).UTC()
						traffic.OnlineAt = &t
					}
				}
				if u.Status == "ACTIVE" && u.TrafficLimitBytes > 0 && traffic.UsedTrafficBytes >= u.TrafficLimitBytes {
					if err := tx.Model(&u).Update("status", "LIMITED").Error; err != nil {
						return err
					}
				}
				if err := importSave(tx, &traffic); err != nil {
					return err
				}
				if err := importSave(tx, &database.InternalSquadMember{InternalSquadUUID: squad.UUID, UserID: u.ID}); err != nil {
					return err
				}
			}
		}
		fmt.Println("[+] 3x-ui clients, traffic, inbound profiles and squads imported. Configure GoWave nodes and public hosts before enabling service. Panel logins, IP limits and panel settings require manual conversion.")
		return nil
	})
}
func Import3xUIInteractive(db *gorm.DB, reader *bufio.Reader) {
	fmt.Print("3x-ui database path (ENTER: /etc/x-ui/x-ui.db): ")
	source, _ := reader.ReadString('\n')
	if err := ImportFrom3xUI(db, strings.TrimSpace(source)); err != nil {
		fmt.Printf("[-] Import failed: %v\n", err)
	}
}
