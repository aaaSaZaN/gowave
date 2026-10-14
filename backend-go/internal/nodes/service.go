package nodes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"remnawave-go/internal/database"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func NanoToMultiplier(v int64) float64 {
	if v == 0 {
		return 1.0
	}
	if v < 1000000 {
		return float64(v)
	}
	return float64(v) / 1e9
}

func MultiplierToNano(v float64) int64 {
	if v <= 0 {
		return 1000000000
	}
	return int64(math.Round(v * 1e9))
}

type NodeHotMetrics struct {
	System      *NodeSystemStatsResponse `json:"system"`
	Versions    *NodeVersionsResponse    `json:"versions"`
	XrayUptime  int64                    `json:"xrayUptime"`
	OnlineUsers int                      `json:"usersOnline"`
	LastSeen    time.Time                `json:"lastSeen"`
}

type NodeVersionsResponse struct {
	Xray     string `json:"xray"`
	Node     string `json:"node"`
	IsCustom bool   `json:"isCustom"`
	NodeType string `json:"nodeType"`
}

type Service struct {
	db          *gorm.DB
	client      *Client
	metricsMu   sync.RWMutex
	nodeMetrics map[string]*NodeHotMetrics
}

func NewService(db *gorm.DB, client *Client) *Service {
	s := &Service{
		db:          db,
		client:      client,
		nodeMetrics: make(map[string]*NodeHotMetrics),
	}
	s.initMetricsFromDB()
	return s
}

func (s *Service) initMetricsFromDB() {
	var metas []database.NodeMeta
	if err := s.db.Find(&metas).Error; err == nil {
		s.metricsMu.Lock()
		defer s.metricsMu.Unlock()
		for _, m := range metas {
			if m.Metadata == "" {
				continue
			}
			var hot NodeHotMetrics
			if err := json.Unmarshal([]byte(m.Metadata), &hot); err == nil {
				var node database.Node
				if err := s.db.Select("uuid").Where("id = ?", m.NodeID).First(&node).Error; err == nil && node.UUID != "" {
					s.nodeMetrics[node.UUID] = &hot
				}
			}
		}
	}
}

func (s *Service) UpdateNodeMetrics(nodeUUID string, fn func(m *NodeHotMetrics)) {
	s.metricsMu.Lock()
	defer s.metricsMu.Unlock()
	m := s.nodeMetrics[nodeUUID]
	if m == nil {
		m = &NodeHotMetrics{}
		s.nodeMetrics[nodeUUID] = m
	}
	fn(m)
}

func (s *Service) Client() *Client {
	s.metricsMu.RLock()
	if s.client != nil {
		s.metricsMu.RUnlock()
		return s.client
	}
	s.metricsMu.RUnlock()

	s.metricsMu.Lock()
	defer s.metricsMu.Unlock()
	if s.client != nil {
		return s.client
	}
	var k database.Keygen
	if err := s.db.Order("created_at asc").First(&k).Error; err == nil && len(k.CACert) > 0 {
		if c, err := NewNodeClient([]byte(k.CACert), []byte(k.ClientCert), []byte(k.ClientKey), []byte(k.PrivKey), []byte(k.PubKey)); err == nil {
			s.client = c
			return c
		}
	}
	return nil
}

func isSystemOrUnsecureProtocol(proto string) bool {
	switch strings.ToLower(proto) {
	case "dokodemo-door", "http", "mixed", "tun", "tunnel", "wireguard", "dns", "api", "metrics":
		return true
	default:
		return false
	}
}

func (s *Service) GetNodeActiveInboundTags(node *database.Node) []string {
	var activeTags []string
	s.db.Table("config_profile_inbounds_to_nodes").
		Select("config_profile_inbounds.tag").
		Joins("JOIN config_profile_inbounds ON config_profile_inbounds_to_nodes.config_profile_inbound_uuid = config_profile_inbounds.uuid").
		Where("config_profile_inbounds_to_nodes.node_uuid = ?", node.UUID).
		Pluck("tag", &activeTags)

	if len(activeTags) == 0 && node.ActiveConfigProfileUUID != nil && *node.ActiveConfigProfileUUID != "" {
		s.db.Table("config_profile_inbounds").
			Where("profile_uuid = ?", *node.ActiveConfigProfileUUID).
			Pluck("tag", &activeTags)
	}
	return activeTags
}

func (s *Service) GetNodeMetrics(nodeUUID string) *NodeHotMetrics {
	s.metricsMu.RLock()
	defer s.metricsMu.RUnlock()
	return s.nodeMetrics[nodeUUID]
}

func (s *Service) SetNodeMetrics(nodeUUID string, m *NodeHotMetrics) {
	s.metricsMu.Lock()
	defer s.metricsMu.Unlock()
	s.nodeMetrics[nodeUUID] = m
}

func (s *Service) DB() *gorm.DB {
	return s.db
}

type CreateNodeDTO struct {
	Name                      string   `json:"name"`
	Address                   string   `json:"address"`
	Port                      *int     `json:"port"`
	CountryCode               string   `json:"countryCode"`
	TrafficLimitBytes         uint64   `json:"trafficLimitBytes"`
	ConsumptionMultiplier     *float64 `json:"consumptionMultiplier"`
	NodeConsumptionMultiplier *float64 `json:"nodeConsumptionMultiplier"`
	ActiveConfigProfileUUID   *string  `json:"activeConfigProfileUuid"`
	ActivePluginUUID          *string  `json:"activePluginUuid"`
	ProviderUUID              *string  `json:"providerUuid"`
	Note                      *string  `json:"note"`
	ConfigProfile             *struct {
		ActiveConfigProfileUUID string   `json:"activeConfigProfileUuid"`
		ActiveInbounds          []string `json:"activeInbounds"`
	} `json:"configProfile"`
}

func (s *Service) CheckNodeHealth(node *database.Node) (bool, string) {
	client := s.Client()
	if client == nil {
		return false, "node client not configured"
	}
	wasConnected := node.IsConnected
	hr, err := client.CheckHealth(node)
	alive := hr.IsAlive
	msg := hr.StatusMsg
	now := time.Now().UTC()
	updates := map[string]interface{}{
		"is_connected":        alive,
		"last_status_change":  &now,
		"last_status_message": nil,
	}
	if err != nil {
		errMsg := err.Error()
		updates["last_status_message"] = &errMsg
	} else if !alive {
		updates["last_status_message"] = &msg
	}
	s.db.Model(&database.Node{}).Where("uuid = ?", node.UUID).Updates(updates)
	node.IsConnected = alive

	nodeVer := hr.NodeVersion
	if nodeVer != "" {
		if !strings.HasPrefix(nodeVer, "v") {
			nodeVer = "v" + nodeVer
		}
		if hr.NodeType == "go" {
			if !strings.Contains(nodeVer, "Go") {
				nodeVer = nodeVer + " (Go)"
			}
		} else {
			if !strings.Contains(nodeVer, "TS") {
				nodeVer = nodeVer + " (TS)"
			}
		}
	}
	xrayVer := hr.XrayVersion
	if xrayVer != "" && !strings.HasPrefix(xrayVer, "v") {
		xrayVer = "v" + xrayVer
	}

	var versionsResp *NodeVersionsResponse
	if nodeVer != "" || xrayVer != "" || hr.NodeType != "" {
		versionsResp = &NodeVersionsResponse{
			Xray:     xrayVer,
			Node:     nodeVer,
			IsCustom: hr.IsCustomCore,
			NodeType: hr.NodeType,
		}
	}

	var toSave *NodeHotMetrics
	s.UpdateNodeMetrics(node.UUID, func(m *NodeHotMetrics) {
		if !alive {
			m.XrayUptime = 0
			m.OnlineUsers = 0
		}
		if versionsResp != nil {
			m.Versions = versionsResp
		}
		toSave = m
	})

	if versionsResp != nil && node.ID > 0 && toSave != nil {
		if data, err := json.Marshal(toSave); err == nil {
			s.db.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "node_id"}},
				DoUpdates: clause.AssignmentColumns([]string{"metadata"}),
			}).Create(&database.NodeMeta{
				NodeID:   node.ID,
				Metadata: string(data),
			})
		}
	}

	if !alive {
		if !node.IsDisabled && err == nil {
			go s.StartNode(node, false)
		}
	} else {
		// If node reconnected or Xray freshly started, ensure all users are synced
		m := s.GetNodeMetrics(node.UUID)
		uptime := int64(0)
		if m != nil {
			uptime = m.XrayUptime
		}
		if !wasConnected || (uptime > 0 && uptime < 35) {
			nodeCopy := *node
			go s.SyncAllUsersToNode(&nodeCopy)
		}
	}

	return alive, msg
}

func (s *Service) StartNode(node *database.Node, force bool) (bool, error) {
	client := s.Client()
	if client == nil {
		return false, errors.New("node client not configured")
	}

	if node.IsDisabled {
		return false, errors.New("node is disabled")
	}

	s.db.Model(&database.Node{}).Where("uuid = ?", node.UUID).Update("is_connecting", true)

	configMap := map[string]interface{}{}
	profileUUID := ""
	if node.ActiveConfigProfileUUID != nil && *node.ActiveConfigProfileUUID != "" {
		profileUUID = *node.ActiveConfigProfileUUID
	} else {
		s.db.Table("config_profile_inbounds_to_nodes").
			Select("config_profile_inbounds.profile_uuid").
			Joins("JOIN config_profile_inbounds ON config_profile_inbounds_to_nodes.config_profile_inbound_uuid = config_profile_inbounds.uuid").
			Where("config_profile_inbounds_to_nodes.node_uuid = ?", node.UUID).
			Limit(1).
			Pluck("profile_uuid", &profileUUID)
	}

	if profileUUID != "" {
		var cp database.ConfigProfile
		if err := s.db.Where("uuid = ?", profileUUID).First(&cp).Error; err == nil {
			_ = json.Unmarshal([]byte(cp.Config), &configMap)
		}
	}

	activeTags := s.GetNodeActiveInboundTags(node)
	if len(activeTags) > 0 {
		activeTagMap := make(map[string]bool, len(activeTags))
		for _, t := range activeTags {
			activeTagMap[t] = true
		}
		if rawInbounds, ok := configMap["inbounds"].([]interface{}); ok {
			filtered := make([]interface{}, 0, len(rawInbounds))
			for _, item := range rawInbounds {
				if ib, ok := item.(map[string]interface{}); ok {
					tag, _ := ib["tag"].(string)
					protocol, _ := ib["protocol"].(string)
					if activeTagMap[tag] || isSystemOrUnsecureProtocol(protocol) {
						filtered = append(filtered, ib)
					}
				} else {
					filtered = append(filtered, item)
				}
			}
			configMap["inbounds"] = filtered
		}
	}

	tags := []string{}
	if len(node.Tags) > 0 {
		tags = []string(node.Tags)
	}

	payload := map[string]interface{}{
		"xrayConfig": configMap,
		"internals": map[string]interface{}{
			"forceRestart": force,
			"hashes": map[string]interface{}{
				"emptyConfig": "hash",
				"inbounds":    []map[string]interface{}{},
			},
			"metadata": map[string]interface{}{
				"uuid":        node.UUID,
				"name":        node.Name,
				"countryCode": node.CountryCode,
				"id":          node.ID,
				"tags":        tags,
			},
		},
	}

	// Sync active plugin if configured
	if node.ActivePluginUUID != nil && *node.ActivePluginUUID != "" {
		var np database.NodePlugin
		if err := s.db.Where("uuid = ?", *node.ActivePluginUUID).First(&np).Error; err == nil {
			var pluginCfg map[string]interface{}
			_ = json.Unmarshal([]byte(np.PluginConfig), &pluginCfg)
			syncPayload := map[string]interface{}{
				"plugin": map[string]interface{}{
					"uuid":   np.UUID,
					"name":   np.Name,
					"config": pluginCfg,
				},
			}
			_, _ = client.SyncPlugin(node, syncPayload)
		}
	}

	ok, err := client.StartXray(node, payload)
	now := time.Now().UTC()
	updates := map[string]interface{}{
		"is_connected":        ok,
		"is_connecting":       false,
		"last_status_change":  &now,
		"last_status_message": nil,
	}
	if err != nil {
		msg := err.Error()
		updates["last_status_message"] = &msg
	}
	s.db.Model(&database.Node{}).Where("uuid = ?", node.UUID).Updates(updates)

	if !ok || err != nil {
		s.UpdateNodeMetrics(node.UUID, func(m *NodeHotMetrics) {
			m.XrayUptime = 0
			m.OnlineUsers = 0
		})
	}

	if ok {
		nodeCopy := *node
		nodeCopy.IsConnected = true
		nodeCopy.IsConnecting = false
		go func(n database.Node) {
			s.CheckNodeHealth(&n)
			time.Sleep(1 * time.Second)
			s.SyncAllUsersToNode(&n)
		}(nodeCopy)
	}

	return ok, err
}

func (s *Service) RestartNodesByProfile(profileUUID string) {
	client := s.Client()
	if client == nil || profileUUID == "" {
		return
	}
	var nodesList []database.Node
	err := s.db.Where(
		"(is_disabled = ? OR is_disabled IS NULL) AND (active_config_profile_uuid = ? OR uuid IN ("+
			"SELECT cpin.node_uuid FROM config_profile_inbounds_to_nodes cpin "+
			"JOIN config_profile_inbounds cpi ON cpi.uuid = cpin.config_profile_inbound_uuid "+
			"WHERE cpi.profile_uuid = ?))",
		false, profileUUID, profileUUID,
	).Find(&nodesList).Error
	if err != nil {
		log.Printf("[NODES] RestartNodesByProfile query error: %v", err)
		return
	}
	for i := range nodesList {
		node := nodesList[i]
		if node.ActiveConfigProfileUUID == nil || *node.ActiveConfigProfileUUID == "" {
			node.ActiveConfigProfileUUID = &profileUUID
		}
		go s.StartNode(&node, true)
	}
}

func (s *Service) PollNodeStats(node *database.Node) {
	client := s.Client()
	if client == nil || !node.IsConnected {
		return
	}

	metrics := s.GetNodeMetrics(node.UUID)
	if metrics == nil {
		metrics = &NodeHotMetrics{}
	}

	if metrics.Versions == nil {
		go s.CheckNodeHealth(node)
	}

	// 1. Fetch system stats
	sysStats, err := client.GetSystemStats(node)
	if err == nil && sysStats != nil {
		metrics.System = sysStats
		if sysStats.XrayInfo != nil {
			metrics.XrayUptime = sysStats.XrayInfo.Uptime
		}
	}

	// 2. Fetch users stats (traffic delta)
	now := time.Now().UTC()
	usersTraffic, err := client.GetUsersStats(node, true)
	activeCount := 0
	if err == nil {

		for _, ut := range usersTraffic {
			delta := ut.Uplink + ut.Downlink
			if delta <= 0 {
				continue
			}
			activeCount++

			// Apply node consumption multiplier (stored as nano: 1.0 = 1,000,000,000)
			mult := NanoToMultiplier(node.ConsumptionMultiplier)
			if mult <= 0 {
				mult = 1.0
			}
			incTraffic := uint64(float64(delta) * mult)

			uid, err := strconv.ParseInt(ut.Username, 10, 64)
			if err != nil || uid <= 0 {
				continue
			}

			// Update user_traffic in database (ut.Username is the user ID string)
			if err := s.db.Exec(`
				UPDATE user_traffic
				SET
					used_traffic_bytes = used_traffic_bytes + ?,
					lifetime_used_traffic_bytes = lifetime_used_traffic_bytes + ?,
					online_at = ?,
					first_connected_at = COALESCE(first_connected_at, ?),
					last_connected_node_uuid = ?
				WHERE id = ?
			`, incTraffic, incTraffic, now, now, node.UUID, uid).Error; err != nil {
				log.Printf("[NODE STATS] update user traffic node=%s user=%d: %v", node.UUID, uid, err)
			}

			// Record user traffic on this node in nodes_user_usage_history
			day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
			history := database.NodesUserUsageHistory{
				NodeID: node.ID, UserID: uint64(uid), TotalBytes: uint64(delta), CreatedAt: day, UpdatedAt: now,
			}
			if err := s.db.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "node_id"}, {Name: "created_at"}, {Name: "user_id"}},
				DoUpdates: clause.Assignments(map[string]interface{}{
					"total_bytes": gorm.Expr("total_bytes + excluded.total_bytes"),
					"updated_at":  gorm.Expr("excluded.updated_at"),
				}),
			}).Create(&history).Error; err != nil {
				log.Printf("[NODE STATS] upsert user usage history node=%s user=%d: %v", node.UUID, uid, err)
			}
		}

		if len(usersTraffic) > 0 {
			metrics.OnlineUsers = activeCount
		}
	}

	// 3. Fetch combined stats (outbounds / inbounds) for node usage history
	combinedStats, err := client.GetCombinedStats(node, true)
	var nodeDownlink, nodeUplink int64
	if err == nil && combinedStats != nil {
		for _, ob := range combinedStats.Outbounds {
			nodeDownlink += ob.Downlink
			nodeUplink += ob.Uplink
		}
		// Fallback to inbounds if outbounds is 0
		if nodeDownlink == 0 && nodeUplink == 0 {
			for _, ib := range combinedStats.Inbounds {
				nodeDownlink += ib.Downlink
				nodeUplink += ib.Uplink
			}
		}
	}
	// Fallback to sum of users traffic deltas if combinedStats returned 0 or failed
	if nodeDownlink == 0 && nodeUplink == 0 && len(usersTraffic) > 0 {
		for _, ut := range usersTraffic {
			if ut.Downlink > 0 || ut.Uplink > 0 {
				nodeDownlink += ut.Downlink
				nodeUplink += ut.Uplink
			}
		}
	}

	nodeTotal := nodeDownlink + nodeUplink
	if nodeTotal > 0 {
		// Record hourly node usage
		hour := now.Truncate(time.Hour)
		history := database.NodesUsageHistory{
			NodeUUID: node.UUID, DownloadBytes: uint64(nodeDownlink), UploadBytes: uint64(nodeUplink),
			TotalBytes: uint64(nodeTotal), CreatedAt: hour, UpdatedAt: now,
		}
		if err := s.db.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "node_uuid"}, {Name: "created_at"}},
			DoUpdates: clause.Assignments(map[string]interface{}{
				"download_bytes": gorm.Expr("download_bytes + excluded.download_bytes"),
				"upload_bytes":   gorm.Expr("upload_bytes + excluded.upload_bytes"),
				"total_bytes":    gorm.Expr("total_bytes + excluded.total_bytes"),
				"updated_at":     gorm.Expr("excluded.updated_at"),
			}),
		}).Create(&history).Error; err != nil {
			log.Printf("[NODE STATS] upsert node usage history node=%s: %v", node.UUID, err)
		}

		// Increment node lifetime traffic with consumption multiplier
		mult := NanoToMultiplier(node.ConsumptionMultiplier)
		if mult <= 0 {
			mult = 1.0
		}
		incNodeTraffic := int64(float64(nodeTotal) * mult)
		if err := s.db.Exec(`UPDATE nodes SET traffic_used_bytes = traffic_used_bytes + ? WHERE uuid = ?`, incNodeTraffic, node.UUID).Error; err != nil {
			log.Printf("[NODE STATS] update node traffic node=%s: %v", node.UUID, err)
		}
	}

	// 4. If online users is still 0, check active IP sessions for accuracy
	onlineUsers := activeCount
	if onlineUsers == 0 {
		sessions, err := client.GetUsersIpList(node)
		if err == nil && len(sessions) > 0 {
			onlineUsers = len(sessions)
			now := time.Now().UTC()
			for _, sess := range sessions {
				if len(sess.IPs) > 0 {
					s.db.Exec(`
						UPDATE user_traffic
						SET online_at = ?,
						    last_connected_node_uuid = ?
						WHERE id = ?
					`, now, node.UUID, sess.UserID)
				}
			}
		}
	}

	s.UpdateNodeMetrics(node.UUID, func(m *NodeHotMetrics) {
		if sysStats != nil {
			m.System = sysStats
			if sysStats.XrayInfo != nil {
				m.XrayUptime = sysStats.XrayInfo.Uptime
			}
		}
		if len(usersTraffic) > 0 || onlineUsers > 0 {
			m.OnlineUsers = onlineUsers
		}
		m.LastSeen = time.Now()
	})
}

func (s *Service) StartHealthCheckLoop(ctx context.Context) {
	healthTicker := time.NewTicker(15 * time.Second)
	statsTicker := time.NewTicker(5 * time.Second)
	defer healthTicker.Stop()
	defer statsTicker.Stop()

	// Initial run
	var nodes []database.Node
	if err := s.db.Where("is_disabled = ?", false).Find(&nodes).Error; err == nil {
		for _, n := range nodes {
			nodeCopy := n
			go func(nc database.Node) {
				s.CheckNodeHealth(&nc)
				s.PollNodeStats(&nc)
			}(nodeCopy)
		}
	}

	for {
		select {
		case <-ctx.Done():
			return
		case <-healthTicker.C:
			var nodes []database.Node
			if err := s.db.Where("is_disabled = ?", false).Find(&nodes).Error; err == nil {
				for _, n := range nodes {
					s.CheckNodeHealth(&n)
				}
			}
		case <-statsTicker.C:
			var nodes []database.Node
			if err := s.db.Where("is_disabled = ? AND is_connected = ?", false, true).Find(&nodes).Error; err == nil {
				for _, n := range nodes {
					go s.PollNodeStats(&n)
				}
			}
		}
	}
}

func (s *Service) Create(dto CreateNodeDTO) (*database.Node, error) {
	port := 443
	if dto.Port != nil && *dto.Port > 0 {
		port = *dto.Port
	}
	if dto.CountryCode == "" {
		dto.CountryCode = "XX"
	}

	var maxPos int
	s.db.Model(&database.Node{}).Select("COALESCE(MAX(view_position), 0)").Scan(&maxPos)

	profileUuid := dto.ActiveConfigProfileUUID
	if dto.ConfigProfile != nil && dto.ConfigProfile.ActiveConfigProfileUUID != "" {
		profileUuid = &dto.ConfigProfile.ActiveConfigProfileUUID
	}

	cm := 1.0
	if dto.ConsumptionMultiplier != nil {
		cm = *dto.ConsumptionMultiplier
	}
	ncm := 1.0
	if dto.NodeConsumptionMultiplier != nil {
		ncm = *dto.NodeConsumptionMultiplier
	}

	node := &database.Node{
		UUID:                      uuid.NewString(),
		Name:                      dto.Name,
		Address:                   dto.Address,
		Port:                      &port,
		CountryCode:               dto.CountryCode,
		TrafficLimitBytes:         dto.TrafficLimitBytes,
		ActiveConfigProfileUUID:   profileUuid,
		ActivePluginUUID:          dto.ActivePluginUUID,
		ProviderUUID:              dto.ProviderUUID,
		Note:                      dto.Note,
		IsConnected:               false,
		IsDisabled:                false,
		ViewPosition:              maxPos + 1,
		ConsumptionMultiplier:     MultiplierToNano(cm),
		NodeConsumptionMultiplier: MultiplierToNano(ncm),
		TrafficResetDay:           1,
		NotifyPercent:             80,
		Tags:                      database.StringArray{},
		IntegrationUUIDs:          database.StringArray{},
		IPs:                       "[]",
		CreatedAt:                 time.Now().UTC(),
		UpdatedAt:                 time.Now().UTC(),
	}

	var activeInbounds []string
	if dto.ConfigProfile != nil {
		activeInbounds = dto.ConfigProfile.ActiveInbounds
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(node).Error; err != nil {
			return err
		}
		profileID := ""
		if profileUuid != nil {
			profileID = *profileUuid
		}
		return createNodeInboundLinks(tx, node.UUID, profileID, activeInbounds)
	}); err != nil {
		return nil, err
	}

	if s.Client() != nil {
		go s.CheckNodeHealth(node)
	}

	return node, nil
}

func (s *Service) GetAll() ([]database.Node, error) {
	var nodes []database.Node
	err := s.db.Order("view_position asc, created_at asc").Find(&nodes).Error
	return nodes, err
}

func (s *Service) GetByUUID(uuid string) (*database.Node, error) {
	var node database.Node
	err := s.db.Where("uuid = ?", uuid).First(&node).Error
	if err != nil {
		return nil, err
	}
	return &node, nil
}

func (s *Service) Update(uuid string, updates map[string]interface{}) (*database.Node, error) {
	updates["updated_at"] = time.Now().UTC()
	err := s.db.Model(&database.Node{}).Where("uuid = ?", uuid).Updates(updates).Error
	if err != nil {
		return nil, err
	}
	return s.GetByUUID(uuid)
}

func (s *Service) UpdateWithConfigProfile(uuid string, updates map[string]interface{}, changeProfile bool, profileUUID *string, inboundUUIDs []string) (*database.Node, error) {
	updates["updated_at"] = time.Now().UTC()
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := updateNodeWithProfile(tx, uuid, updates, changeProfile, profileUUID, inboundUUIDs); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.GetByUUID(uuid)
}

func (s *Service) BulkUpdateConfigProfile(nodeUUIDs []string, profileUUID *string, inboundUUIDs []string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		seen := make(map[string]struct{}, len(nodeUUIDs))
		for _, nodeUUID := range nodeUUIDs {
			if nodeUUID == "" {
				continue
			}
			if _, exists := seen[nodeUUID]; exists {
				continue
			}
			seen[nodeUUID] = struct{}{}
			if err := updateNodeWithProfile(tx, nodeUUID, map[string]interface{}{}, true, profileUUID, inboundUUIDs); err != nil {
				return fmt.Errorf("update node %s: %w", nodeUUID, err)
			}
		}
		return nil
	})
}

func updateNodeWithProfile(tx *gorm.DB, nodeUUID string, updates map[string]interface{}, changeProfile bool, profileUUID *string, inboundUUIDs []string) error {
	var node database.Node
	if err := tx.Where("uuid = ?", nodeUUID).First(&node).Error; err != nil {
		return err
	}
	if changeProfile {
		if profileUUID != nil && strings.TrimSpace(*profileUUID) != "" {
			cleanProfileUUID := strings.TrimSpace(*profileUUID)
			var profile database.ConfigProfile
			if err := tx.Select("uuid").Where("uuid = ?", cleanProfileUUID).First(&profile).Error; err != nil {
				return fmt.Errorf("config profile %s does not exist: %w", cleanProfileUUID, err)
			}
			updates["active_config_profile_uuid"] = cleanProfileUUID
		} else {
			updates["active_config_profile_uuid"] = nil
		}
	}
	updates["updated_at"] = time.Now().UTC()
	if err := tx.Model(&database.Node{}).Where("uuid = ?", nodeUUID).Updates(updates).Error; err != nil {
		return err
	}
	if !changeProfile {
		return nil
	}
	if err := tx.Where("node_uuid = ?", nodeUUID).Delete(&database.ConfigProfileInboundsToNodes{}).Error; err != nil {
		return err
	}
	if profileUUID == nil || strings.TrimSpace(*profileUUID) == "" {
		if len(inboundUUIDs) > 0 {
			return fmt.Errorf("cannot attach inbounds without an active config profile")
		}
		return nil
	}
	return createNodeInboundLinks(tx, nodeUUID, strings.TrimSpace(*profileUUID), inboundUUIDs)
}

func createNodeInboundLinks(tx *gorm.DB, nodeUUID, profileID string, inboundUUIDs []string) error {
	uniqueUUIDs := make([]string, 0, len(inboundUUIDs))
	seen := make(map[string]struct{}, len(inboundUUIDs))
	for _, inboundUUID := range inboundUUIDs {
		if inboundUUID == "" {
			continue
		}
		if _, exists := seen[inboundUUID]; exists {
			continue
		}
		seen[inboundUUID] = struct{}{}
		uniqueUUIDs = append(uniqueUUIDs, inboundUUID)
	}
	if len(uniqueUUIDs) == 0 {
		return nil
	}
	if strings.TrimSpace(profileID) == "" {
		return fmt.Errorf("cannot attach inbounds without an active config profile")
	}
	var inbounds []database.ConfigProfileInbound
	if err := tx.Select("uuid", "profile_uuid").Where("uuid IN ?", uniqueUUIDs).Find(&inbounds).Error; err != nil {
		return err
	}
	if len(inbounds) != len(uniqueUUIDs) {
		return fmt.Errorf("one or more inbounds do not exist")
	}
	links := make([]database.ConfigProfileInboundsToNodes, 0, len(uniqueUUIDs))
	for _, inbound := range inbounds {
		if inbound.ProfileUUID != profileID {
			return fmt.Errorf("inbound %s does not belong to config profile %s", inbound.UUID, profileID)
		}
		links = append(links, database.ConfigProfileInboundsToNodes{
			ConfigProfileInboundUUID: inbound.UUID,
			NodeUUID:                 nodeUUID,
		})
	}
	return tx.Create(&links).Error
}

func (s *Service) Delete(uuid string) error {
	return s.db.Where("uuid = ?", uuid).Delete(&database.Node{}).Error
}

type InboundInfo struct {
	Tag        string `gorm:"column:tag"`
	Type       string `gorm:"column:type"`
	Network    string `gorm:"column:network"`
	Security   string `gorm:"column:security"`
	RawInbound string `gorm:"column:raw_inbound"`
}

func getVlessFlow(network, security, rawInbound string) string {
	if rawInbound != "" {
		var raw map[string]interface{}
		if err := json.Unmarshal([]byte(rawInbound), &raw); err == nil {
			if settings, ok := raw["settings"].(map[string]interface{}); ok {
				if f, ok := settings["flow"].(string); ok {
					return f
				}
			}
		}
	}
	if (network == "tcp" || network == "raw") && (security == "reality" || security == "tls") {
		return "xtls-rprx-vision"
	}
	return ""
}

func isSS2022(rawInbound string) bool {
	if rawInbound == "" {
		return false
	}
	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(rawInbound), &raw); err == nil {
		if settings, ok := raw["settings"].(map[string]interface{}); ok {
			if method, ok := settings["method"].(string); ok {
				return strings.HasPrefix(method, "2022-")
			}
		}
	}
	return false
}

func (s *Service) GetUserResolvedInbounds(userID uint64) ([]InboundInfo, error) {
	var inbounds []InboundInfo
	err := s.db.Table("internal_squad_members").
		Select("config_profile_inbounds.tag, config_profile_inbounds.type, config_profile_inbounds.network, config_profile_inbounds.security, config_profile_inbounds.raw_inbound").
		Joins("JOIN internal_squads ON internal_squad_members.internal_squad_uuid = internal_squads.uuid").
		Joins("JOIN internal_squad_inbounds ON internal_squads.uuid = internal_squad_inbounds.internal_squad_uuid").
		Joins("JOIN config_profile_inbounds ON internal_squad_inbounds.inbound_uuid = config_profile_inbounds.uuid").
		Where("internal_squad_members.user_id = ?", userID).
		Scan(&inbounds).Error
	return inbounds, err
}

func (s *Service) syncUserToSingleNode(node *database.Node, user *database.User, prevVlessUUID *string, inbounds []InboundInfo) {
	client := s.Client()
	if client == nil || user == nil || node == nil {
		return
	}
	isActive := strings.ToUpper(user.Status) == "ACTIVE"

	activeTags := s.GetNodeActiveInboundTags(node)
	tagSet := make(map[string]bool)
	for _, t := range activeTags {
		tagSet[t] = true
	}

	var matchedData []map[string]interface{}
	if isActive {
		for _, inb := range inbounds {
			if !tagSet[inb.Tag] {
				continue
			}
			inbType := inb.Type
			if inbType == "shadowsocks" && isSS2022(inb.RawInbound) {
				inbType = "shadowsocks22"
			}
			uID := strconv.FormatUint(user.ID, 10)
			switch inbType {
			case "vless":
				flow := getVlessFlow(inb.Network, inb.Security, inb.RawInbound)
				matchedData = append(matchedData, map[string]interface{}{
					"type":     "vless",
					"tag":      inb.Tag,
					"username": uID,
					"uuid":     user.VlessUUID,
					"flow":     flow,
				})
			case "hysteria":
				matchedData = append(matchedData, map[string]interface{}{
					"type":     "hysteria",
					"tag":      inb.Tag,
					"username": uID,
					"password": user.VlessUUID,
				})
			case "trojan":
				matchedData = append(matchedData, map[string]interface{}{
					"type":     "trojan",
					"tag":      inb.Tag,
					"username": uID,
					"password": user.TrojanPassword,
				})
			case "shadowsocks", "shadowsocks22":
				matchedData = append(matchedData, map[string]interface{}{
					"type":     inbType,
					"tag":      inb.Tag,
					"username": uID,
					"password": user.SsPassword,
				})
			}
		}
	}

	if len(matchedData) > 0 {
		req := AddUserRequestPayload{
			HashData: NodeUserHashData{
				VlessUUID:     user.VlessUUID,
				PrevVlessUUID: prevVlessUUID,
			},
			Data: matchedData,
		}
		go func(n database.Node, r AddUserRequestPayload) {
			if err := client.AddUser(&n, r); err != nil {
				log.Printf("[NODES] Failed to sync user id=%d to node %q: %v", user.ID, n.Name, err)
			}
		}(*node, req)
	} else {
		req := RemoveUserRequestPayload{
			Username: strconv.FormatUint(user.ID, 10),
			HashData: NodeUserHashData{
				VlessUUID: user.VlessUUID,
			},
		}
		go func(n database.Node, r RemoveUserRequestPayload) {
			if err := client.RemoveUser(&n, r); err != nil {
				log.Printf("[NODES] Failed to remove user id=%d from node %q: %v", user.ID, n.Name, err)
			}
		}(*node, req)
	}
}

func (s *Service) SyncAllUsersToNode(node *database.Node) {
	if s.Client() == nil || node == nil {
		return
	}
	var users []database.User
	if err := s.db.Where("status = ?", "ACTIVE").Find(&users).Error; err != nil {
		return
	}
	for i := range users {
		inbounds, err := s.GetUserResolvedInbounds(users[i].ID)
		if err != nil {
			log.Printf("[NODES] Failed to resolve inbounds for user id=%d: %v", users[i].ID, err)
			continue
		}
		s.syncUserToSingleNode(node, &users[i], nil, inbounds)
	}
}

func (s *Service) SyncUserToNodes(user *database.User, prevVlessUUID *string) {
	if s.Client() == nil || user == nil {
		return
	}

	var connectedNodes []database.Node
	if err := s.db.Where("is_connected = ? AND is_disabled = ?", true, false).Find(&connectedNodes).Error; err != nil || len(connectedNodes) == 0 {
		return
	}

	inbounds, err := s.GetUserResolvedInbounds(user.ID)
	if err != nil {
		log.Printf("[NODES] Failed to resolve inbounds for user id=%d: %v", user.ID, err)
		return
	}

	for _, node := range connectedNodes {
		s.syncUserToSingleNode(&node, user, prevVlessUUID, inbounds)
	}
}

func (s *Service) RemoveUserFromNodes(user *database.User) {
	client := s.Client()
	if client == nil || user == nil {
		return
	}

	var connectedNodes []database.Node
	if err := s.db.Where("is_connected = ? AND is_disabled = ?", true, false).Find(&connectedNodes).Error; err != nil || len(connectedNodes) == 0 {
		return
	}

	req := RemoveUserRequestPayload{
		Username: strconv.FormatUint(user.ID, 10),
		HashData: NodeUserHashData{
			VlessUUID: user.VlessUUID,
		},
	}
	for _, node := range connectedNodes {
		go func(n database.Node, r RemoveUserRequestPayload) {
			_ = client.RemoveUser(&n, r)
		}(node, req)
	}
}

func (s *Service) SyncAllUsersToConnectedNodes() {
	var users []database.User
	if err := s.db.Where("status = ?", "ACTIVE").Find(&users).Error; err != nil {
		return
	}
	for i := range users {
		s.SyncUserToNodes(&users[i], nil)
	}
}
