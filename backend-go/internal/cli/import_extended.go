package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gowave/internal/database"
)

// Stable identifiers make re-imports update the same records without borrowing
// source numeric IDs from unrelated destination records.
func importUUID(panel, kind string, id uint64) string {
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte(fmt.Sprintf("gowave/import/%s/%s/%d", panel, kind, id))).String()
}
func importSave(db *gorm.DB, value interface{}) error {
	return db.Clauses(clause.OnConflict{UpdateAll: true}).Create(value).Error
}
func importProfile(db *gorm.DB, panel string, id uint64, name, config string) (map[string]string, error) {
	var cfg struct {
		Inbounds []json.RawMessage `json:"inbounds"`
	}
	if err := json.Unmarshal([]byte(config), &cfg); err != nil {
		return nil, err
	}
	profile := database.ConfigProfile{UUID: importUUID(panel, "profile", id), Name: fmt.Sprintf("%s-%d-%s", panel, id, name), Config: config}
	if err := importSave(db, &profile); err != nil {
		return nil, err
	}
	tags := map[string]string{}
	for i, raw := range cfg.Inbounds {
		var inbound struct {
			Tag            string
			Protocol       string
			Port           int
			StreamSettings struct {
				Network  *string
				Security *string
			}
		}
		if err := json.Unmarshal(raw, &inbound); err != nil {
			return nil, err
		}
		if inbound.Tag == "" {
			return nil, fmt.Errorf("profile %d inbound %d has no tag", id, i)
		}
		b := database.ConfigProfileInbound{UUID: uuid.NewSHA1(uuid.NameSpaceURL, []byte(profile.UUID+"/"+inbound.Tag)).String(), ProfileUUID: profile.UUID, Tag: inbound.Tag, Type: inbound.Protocol, Port: &inbound.Port, Network: inbound.StreamSettings.Network, Security: inbound.StreamSettings.Security, RawInbound: string(raw)}
		if err := importSave(db, &b); err != nil {
			return nil, err
		}
		tags[inbound.Tag] = b.UUID
	}
	return tags, nil
}

func importPasarGuardExtended(dst, src *gorm.DB, userIDs map[uint64]uint64) error {
	tags := map[string]string{}
	var configs []struct {
		ID                 uint64
		Name, Config, Type string
	}
	if src.Migrator().HasTable("core_configs") {
		if err := src.Table("core_configs").Find(&configs).Error; err != nil {
			return err
		}
		for _, c := range configs {
			if c.Type != "" && c.Type != "xray" {
				return fmt.Errorf("unsupported core type %q", c.Type)
			}
			mapped, err := importProfile(dst, "pasarguard", c.ID, c.Name, c.Config)
			if err != nil {
				return err
			}
			for tag, id := range mapped {
				tags[tag] = id
			}
		}
	}
	var nodes []struct {
		ID                          uint64
		Name, Address               string
		Port                        int
		CreatedAt                   time.Time
		Uplink, Downlink, DataLimit uint64
		CoreConfigID                *uint64
		UsageCoefficient            float64
	}
	nodeIDs := map[uint64]uint64{}
	if src.Migrator().HasTable("nodes") {
		if err := src.Table("nodes").Find(&nodes).Error; err != nil {
			return err
		}
		for _, n := range nodes {
			node := database.Node{UUID: importUUID("pasarguard", "node", n.ID), Name: n.Name, Address: n.Address, Port: &n.Port, IsDisabled: true, CreatedAt: n.CreatedAt, TrafficUsedBytes: n.Uplink + n.Downlink, TrafficLimitBytes: n.DataLimit, ConsumptionMultiplier: int64(n.UsageCoefficient * 1e9)}
			if n.CoreConfigID != nil {
				p := importUUID("pasarguard", "profile", *n.CoreConfigID)
				node.ActiveConfigProfileUUID = &p
			}
			var existing database.Node
			err := dst.Where("uuid = ?", node.UUID).First(&existing).Error
			if err == nil {
				node.ID = existing.ID
			} else if err != gorm.ErrRecordNotFound {
				return err
			}
			if err := importSave(dst, &node); err != nil {
				return err
			}
			nodeIDs[n.ID] = node.ID
			if node.ActiveConfigProfileUUID != nil {
				var inbounds []database.ConfigProfileInbound
				if err := dst.Where("profile_uuid = ?", *node.ActiveConfigProfileUUID).Find(&inbounds).Error; err != nil {
					return err
				}
				for _, b := range inbounds {
					if err := importSave(dst, &database.ConfigProfileInboundsToNodes{ConfigProfileInboundUUID: b.UUID, NodeUUID: node.UUID}); err != nil {
						return err
					}
				}
			}
		}
	}
	var groups []struct {
		ID         uint64
		Name       string
		IsDisabled bool
	}
	if src.Migrator().HasTable("groups") {
		if err := src.Table("groups").Find(&groups).Error; err != nil {
			return err
		}
		for _, g := range groups {
			s := database.InternalSquad{UUID: importUUID("pasarguard", "group", g.ID), Name: g.Name}
			if err := importSave(dst, &s); err != nil {
				return err
			}
		}
		var links []struct{ UserID, GroupsID uint64 }
		if err := src.Table("users_groups_association").Find(&links).Error; err != nil {
			return err
		}
		for _, l := range links {
			if userIDs[l.UserID] == 0 {
				return fmt.Errorf("unknown group user %d", l.UserID)
			}
			if err := importSave(dst, &database.InternalSquadMember{InternalSquadUUID: importUUID("pasarguard", "group", l.GroupsID), UserID: userIDs[l.UserID]}); err != nil {
				return err
			}
		}
		var inbounds []struct {
			ID  uint64
			Tag string
		}
		if err := src.Table("inbounds").Find(&inbounds).Error; err != nil {
			return err
		}
		var inboundLinks []struct{ InboundID, GroupID uint64 }
		if err := src.Table("inbounds_groups_association").Find(&inboundLinks).Error; err != nil {
			return err
		}
		for _, l := range inboundLinks {
			for _, b := range inbounds {
				if b.ID == l.InboundID && tags[b.Tag] != "" {
					if err := importSave(dst, &database.InternalSquadInbound{InternalSquadUUID: importUUID("pasarguard", "group", l.GroupID), InboundUUID: tags[b.Tag]}); err != nil {
						return err
					}
				}
			}
		}
	}
	if src.Migrator().HasTable("hosts") {
		var hosts []struct {
			ID                                                                         uint64
			Remark, Address                                                            string
			Port                                                                       int
			InboundTag, Sni, Host, Alpn, Fingerprint, Path, Security                   string
			IsDisabled                                                                 bool
			Priority                                                                   int
			MuxSettings, FinalMaskSettings, PinnedPeerCertSha256, VerifyPeerCertByName string
		}
		if err := src.Table("hosts").Find(&hosts).Error; err != nil {
			return err
		}
		for _, h := range hosts {
			host := database.Host{UUID: importUUID("pasarguard", "host", h.ID), Remark: h.Remark, Address: h.Address, Port: h.Port, Sni: h.Sni, Host: h.Host, Alpn: h.Alpn, Fingerprint: h.Fingerprint, Path: h.Path, IsDisabled: h.IsDisabled, ViewPosition: h.Priority, MuxParams: h.MuxSettings, FinalMask: h.FinalMaskSettings, PinnedPeerCertSha256: h.PinnedPeerCertSha256, VerifyPeerCertByName: h.VerifyPeerCertByName, SecurityLayer: strings.ToUpper(h.Security)}
			if host.SecurityLayer == "INBOUND_DEFAULT" {
				host.SecurityLayer = "DEFAULT"
			}
			if b := tags[h.InboundTag]; b != "" {
				host.ConfigProfileInboundUUID = &b
				var inbound database.ConfigProfileInbound
				if err := dst.First(&inbound, "uuid = ?", b).Error; err != nil {
					return err
				}
				host.ConfigProfileUUID = &inbound.ProfileUUID
			}
			if err := importSave(dst, &host); err != nil {
				return err
			}
		}
	}
	if src.Migrator().HasTable("node_usages") {
		var rows []struct {
			NodeID           uint64
			CreatedAt        time.Time
			Uplink, Downlink uint64
		}
		if err := src.Table("node_usages").Find(&rows).Error; err != nil {
			return err
		}
		for _, r := range rows {
			if nodeIDs[r.NodeID] == 0 {
				continue
			}
			if err := importSave(dst, &database.NodesUsageHistory{NodeUUID: importUUID("pasarguard", "node", r.NodeID), CreatedAt: r.CreatedAt, UploadBytes: r.Uplink, DownloadBytes: r.Downlink, TotalBytes: r.Uplink + r.Downlink}); err != nil {
				return err
			}
		}
	}
	if src.Migrator().HasTable("node_user_usages") {
		var rows []struct {
			NodeID, UserID uint64
			CreatedAt      time.Time
			UsedTraffic    uint64
		}
		if err := src.Table("node_user_usages").Find(&rows).Error; err != nil {
			return err
		}
		for _, r := range rows {
			if nodeIDs[r.NodeID] == 0 {
				continue
			}
			if err := importSave(dst, &database.NodesUserUsageHistory{NodeID: nodeIDs[r.NodeID], UserID: userIDs[r.UserID], CreatedAt: r.CreatedAt, TotalBytes: r.UsedTraffic}); err != nil {
				return err
			}
		}
	}
	if src.Migrator().HasTable("user_usage_logs") {
		var rows []struct {
			UserID             uint64
			UsedTrafficAtReset uint64
			ResetAt            time.Time
		}
		if err := src.Table("user_usage_logs").Find(&rows).Error; err != nil {
			return err
		}
		for _, r := range rows {
			if err := dst.Model(&database.UserTraffic{}).Where("id = ?", userIDs[r.UserID]).UpdateColumn("lifetime_used_traffic_bytes", gorm.Expr("lifetime_used_traffic_bytes + ?", r.UsedTrafficAtReset)).Error; err != nil {
				return err
			}
			if err := dst.Model(&database.User{}).Where("id = ? AND (last_traffic_reset_at IS NULL OR last_traffic_reset_at < ?)", userIDs[r.UserID], r.ResetAt).Update("last_traffic_reset_at", r.ResetAt).Error; err != nil {
				return err
			}
		}
	}
	if src.Migrator().HasTable("user_subscription_updates") {
		var rows []struct {
			ID, UserID    uint64
			CreatedAt     time.Time
			UserAgent, IP string
		}
		if err := src.Table("user_subscription_updates").Find(&rows).Error; err != nil {
			return err
		}
		for _, r := range rows {
			q := database.UserSubscriptionRequestHistory{UserID: userIDs[r.UserID], RequestAt: r.CreatedAt, UserAgent: &r.UserAgent, RequestIP: &r.IP}
			if err := dst.Where("user_id = ? AND request_at = ?", q.UserID, q.RequestAt).FirstOrCreate(&q).Error; err != nil {
				return err
			}
		}
	}
	fmt.Println("[*] Imported compatible profiles, groups, hosts, disabled nodes and traffic/request history. Panel authentication, API keys, notification settings and templates require manual conversion.")
	return nil
}
