package nodes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	prom "gowave/internal/metrics"
	"log"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"gowave/internal/configrender"
	"gowave/internal/database"
	"gowave/internal/jobs"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func NanoToMultiplier(v int64) float64 {
	if v == 0 {
		return 0
	}
	if v < 1000000 {
		return float64(v)
	}
	return float64(v) / 1e9
}

func MultiplierToNano(v float64) int64 {
	if v <= 0 {
		return 0
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
	pollMu      sync.Mutex
	polling     map[string]bool
	usage       UsageOptions
}

func NewService(db *gorm.DB, client *Client) *Service {
	s := &Service{
		db:          db,
		client:      client,
		nodeMetrics: make(map[string]*NodeHotMetrics),
		polling:     make(map[string]bool),
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

// ResetClient drops the cached mTLS client so the next call reloads keys from
// the database, for example after a backup restore replaced the keygen row.
func (s *Service) ResetClient() {
	s.metricsMu.Lock()
	s.client = nil
	s.nodeMetrics = make(map[string]*NodeHotMetrics)
	s.metricsMu.Unlock()
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
	return cloneMetrics(s.nodeMetrics[nodeUUID])
}

func cloneMetrics(m *NodeHotMetrics) *NodeHotMetrics {
	if m == nil {
		return nil
	}
	data, _ := json.Marshal(m)
	var snapshot NodeHotMetrics
	_ = json.Unmarshal(data, &snapshot)
	return &snapshot
}

func (s *Service) SetNodeMetrics(nodeUUID string, m *NodeHotMetrics) {
	s.metricsMu.Lock()
	defer s.metricsMu.Unlock()
	s.nodeMetrics[nodeUUID] = cloneMetrics(m)
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
	ProxyURL                  *string  `json:"proxyUrl"`
	IsTrafficTrackingActive   bool     `json:"isTrafficTrackingActive"`
	NotifyPercent             *int     `json:"notifyPercent"`
	TrafficResetDay           *int     `json:"trafficResetDay"`
	Tags                      []string `json:"tags"`
	IntegrationUUIDs          []string `json:"integrationUuids"`
	IPs                       []NodeIP `json:"ips"`
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
	var stored database.Node
	if err := s.db.Select("is_connected").Where("uuid = ?", node.UUID).First(&stored).Error; err == nil {
		wasConnected = stored.IsConnected
	}
	// Only a state flip notifies, so repeated failed checks stay quiet.
	changed := s.db.Model(&database.Node{}).Where("uuid = ? AND is_disabled = ? AND is_connected = ?", node.UUID, false, wasConnected).Updates(updates)
	if changed.Error != nil {
		return false, "node disabled or unavailable"
	}
	if changed.RowsAffected == 1 && wasConnected != alive {
		event := "node.connection_restored"
		if !alive {
			event = "node.connection_lost"
		}
		if err := s.emitNodeByUUID(s.db, node.UUID, event); err != nil {
			log.Printf("[NODES] %s notification: %v", event, err)
		}
	}
	if changed.RowsAffected != 1 {
		// Another check flipped the state first, or the node is disabled.
		retry := s.db.Model(&database.Node{}).Where("uuid = ? AND is_disabled = ?", node.UUID, false).Updates(updates)
		if retry.Error != nil || retry.RowsAffected != 1 {
			return false, "node disabled or unavailable"
		}
	}
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
		toSave = cloneMetrics(m)
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
			revision := uint64(time.Now().Unix() / 60)
			if err := jobs.Enqueue(s.db, "node-start", node.UUID, node.UUID, revision, map[string]string{"uuid": node.UUID}); err != nil {
				log.Printf("[NODES] enqueue start for %s: %v", node.UUID, err)
			}
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
	startupSuccessful := false
	defer func() {
		if !startupSuccessful {
			s.db.Model(&database.Node{}).Where("uuid = ?", node.UUID).Update("is_connecting", false)
		}
	}()

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
			expanded, err := configrender.Expand(s.db, cp.Config)
			if err != nil {
				return false, err
			}
			configMap = expanded
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

	hashes, err := s.populateConfigUsers(configMap, profileUUID)
	if err != nil {
		return false, err
	}

	payload := map[string]interface{}{
		"xrayConfig": configMap,
		"internals": map[string]interface{}{
			"forceRestart": force,
			"hashes":       hashes,
			"metadata": map[string]interface{}{
				"uuid":        node.UUID,
				"name":        node.Name,
				"countryCode": node.CountryCode,
				"id":          node.ID,
				"tags":        tags,
			},
		},
	}

	integrations := map[string]interface{}{}
	for _, id := range node.IntegrationUUIDs {
		var record database.Integration
		if err := s.db.Where("uuid=?", id).First(&record).Error; err != nil {
			return false, err
		}
		var config map[string]interface{}
		if err := json.Unmarshal([]byte(record.Config), &config); err != nil {
			return false, err
		}
		for key, value := range config {
			integrations[key] = value
		}
	}
	payload["internals"].(map[string]interface{})["integrations"] = integrations

	// Sync active plugin if configured
	if node.ActivePluginUUID != nil && *node.ActivePluginUUID != "" {
		var np database.NodePlugin
		if err := s.db.Where("uuid = ?", *node.ActivePluginUUID).First(&np).Error; err == nil {
			pluginCfg, err := configrender.Plugin(s.db, np.PluginConfig)
			if err != nil {
				return false, err
			}
			syncPayload := map[string]interface{}{
				"plugin": map[string]interface{}{
					"uuid":   np.UUID,
					"name":   np.Name,
					"config": pluginCfg,
				},
			}
			if ok, err := client.SyncPlugin(node, syncPayload); err != nil || !ok {
				if err == nil {
					err = fmt.Errorf("plugin synchronization rejected")
				}
				return false, err
			}
		}
	}

	startupSuccessful = true
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
		if err := jobs.EnqueueNode(s.db, node.UUID, "node-start"); err != nil {
			log.Printf("[NODES] enqueue restart: %v", err)
		}
	}
}

func (s *Service) PollNodeStats(node *database.Node) {
	s.pollMu.Lock()
	if s.polling[node.UUID] {
		s.pollMu.Unlock()
		return
	}
	s.polling[node.UUID] = true
	s.pollMu.Unlock()
	defer func() { s.pollMu.Lock(); delete(s.polling, node.UUID); s.pollMu.Unlock() }()
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
		if sysStats.Plugins != nil && sysStats.Plugins.TorrentBlocker != nil && sysStats.Plugins.TorrentBlocker.ReportsCount > 0 {
			s.enqueueReportCollection(node, time.Now().UTC())
		}
	}

	// 2. Fetch users stats (traffic delta) and account it.
	now := time.Now().UTC()
	usersTraffic, err := client.GetUsersStats(node, true)
	activeCount := 0
	if err == nil {
		activeCount = s.recordUserUsage(node, usersTraffic, now)
		metrics.OnlineUsers = activeCount
	}

	// 3. Node usage ports RecordNodeUsageQueueProcessor: outbound totals feed
	// the hourly history and the node's traffic; nothing is recorded when
	// they are zero. Inbound/outbound counters go to Prometheus.
	if combinedStats, err := client.GetCombinedStats(node, true); err == nil && combinedStats != nil {
		var nodeDownlink, nodeUplink int64
		for _, ob := range combinedStats.Outbounds {
			nodeDownlink += ob.Downlink
			nodeUplink += ob.Uplink
		}
		if nodeTotal := nodeDownlink + nodeUplink; nodeTotal != 0 {
			hour := now.Truncate(time.Hour)
			history := database.NodesUsageHistory{
				NodeUUID: node.UUID, DownloadBytes: uint64(nodeDownlink), UploadBytes: uint64(nodeUplink),
				TotalBytes: uint64(nodeTotal), CreatedAt: hour, UpdatedAt: now,
			}
			if err := s.db.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "node_uuid"}, {Name: "created_at"}},
				DoUpdates: clause.Assignments(map[string]interface{}{
					"download_bytes": gorm.Expr("nodes_usage_history.download_bytes + excluded.download_bytes"),
					"upload_bytes":   gorm.Expr("nodes_usage_history.upload_bytes + excluded.upload_bytes"),
					"total_bytes":    gorm.Expr("nodes_usage_history.total_bytes + excluded.total_bytes"),
					"updated_at":     gorm.Expr("excluded.updated_at"),
				}),
			}).Create(&history).Error; err != nil {
				log.Printf("[NODE STATS] upsert node usage history node=%s: %v", node.UUID, err)
			}

			// Node traffic uses its own multiplier, independent of users'.
			incNodeTraffic := multiplyConsumption(node.NodeConsumptionMultiplier, nodeTotal)
			if err := s.db.Exec(`UPDATE nodes SET traffic_used_bytes = traffic_used_bytes + ? WHERE uuid = ?`, incNodeTraffic, node.UUID).Error; err != nil {
				log.Printf("[NODE STATS] update node traffic node=%s: %v", node.UUID, err)
			}

			inbounds := make([]prom.NodeTraffic, 0, len(combinedStats.Inbounds))
			for _, ib := range combinedStats.Inbounds {
				inbounds = append(inbounds, prom.NodeTraffic{Tag: ib.Inbound, Uplink: float64(ib.Uplink), Downlink: float64(ib.Downlink)})
			}
			outbounds := make([]prom.NodeTraffic, 0, len(combinedStats.Outbounds))
			for _, ob := range combinedStats.Outbounds {
				outbounds = append(outbounds, prom.NodeTraffic{Tag: ob.Outbound, Uplink: float64(ob.Uplink), Downlink: float64(ob.Downlink)})
			}
			prom.RecordNodeTraffic(node.UUID, inbounds, outbounds)
		}
	}
	onlineUsers := activeCount

	s.UpdateNodeMetrics(node.UUID, func(m *NodeHotMetrics) {
		if sysStats != nil {
			m.System = sysStats
			if sysStats.XrayInfo != nil {
				m.XrayUptime = sysStats.XrayInfo.Uptime
			}
		}
		m.OnlineUsers = onlineUsers
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

	notifyPercent, resetDay := 80, 1
	if dto.NotifyPercent != nil {
		if *dto.NotifyPercent < 0 || *dto.NotifyPercent > 100 {
			return nil, fmt.Errorf("notifyPercent must be between 0 and 100")
		}
		notifyPercent = *dto.NotifyPercent
	}
	if dto.TrafficResetDay != nil {
		if *dto.TrafficResetDay < 1 || *dto.TrafficResetDay > 31 {
			return nil, fmt.Errorf("trafficResetDay must be between 1 and 31")
		}
		resetDay = *dto.TrafficResetDay
	}
	ips, err := encodeNodeIPs(dto.IPs)
	if err != nil {
		return nil, err
	}
	tags := database.StringArray(dto.Tags)
	if tags == nil {
		tags = database.StringArray{}
	}
	integrations := database.StringArray(dto.IntegrationUUIDs)
	if integrations == nil {
		integrations = database.StringArray{}
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
		ProxyURL:                  dto.ProxyURL,
		IsTrafficTrackingActive:   dto.IsTrafficTrackingActive,
		TrafficResetDay:           resetDay,
		NotifyPercent:             notifyPercent,
		Tags:                      tags,
		IntegrationUUIDs:          integrations,
		IPs:                       ips,
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
		if err := createNodeInboundLinks(tx, node.UUID, profileID, activeInbounds); err != nil {
			return err
		}
		return s.emitNode(tx, node, "node.created")
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
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&database.Node{}).Where("uuid = ?", uuid).Updates(updates).Error; err != nil {
			return err
		}
		return s.emitNodeByUUID(tx, uuid, "node.modified")
	})
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
		return s.emitNodeByUUID(tx, uuid, "node.modified")
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

// Delete removes a node, queues an Xray stop for the agent and notifies.
func (s *Service) Delete(uuid string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var node database.Node
		if err := tx.Where("uuid = ?", uuid).First(&node).Error; err != nil {
			return err
		}
		if err := s.emitNode(tx, &node, "node.deleted"); err != nil {
			return err
		}
		// The tombstone carries the address so the agent is stopped even
		// though the row is gone.
		if err := jobs.Enqueue(tx, "node-retire", node.UUID, node.UUID, uint64(time.Now().UnixNano()), node); err != nil {
			return err
		}
		return tx.Where("uuid = ?", uuid).Delete(&database.Node{}).Error
	})
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
	if err != nil {
		return nil, err
	}
	// Squads may share inbounds; a repeated tag makes the node reject the
	// whole add-user call with "User already exists".
	seen := make(map[string]bool, len(inbounds))
	unique := inbounds[:0]
	for _, inbound := range inbounds {
		if !seen[inbound.Tag] {
			seen[inbound.Tag] = true
			unique = append(unique, inbound)
		}
	}
	return unique, nil
}

func (s *Service) syncUserToSingleNode(node *database.Node, user *database.User, prevVlessUUID *string, inbounds []InboundInfo) error {
	client := s.Client()
	if client == nil || user == nil || node == nil {
		return errors.New("node client unavailable")
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
		return client.AddUser(node, req)
	} else {
		req := RemoveUserRequestPayload{
			Username: strconv.FormatUint(user.ID, 10),
			HashData: NodeUserHashData{
				VlessUUID: user.VlessUUID,
			},
		}
		return client.RemoveUser(node, req)
	}
}

// SyncAllUsersToNode schedules a fresh reconciliation after a node restart.
func (s *Service) SyncAllUsersToNode(node *database.Node) {
	if node == nil {
		return
	}
	var users []database.User
	if err := s.db.Where("status = ?", "ACTIVE").Find(&users).Error; err != nil {
		log.Printf("[NODES] reconcile: %v", err)
		return
	}
	revision := uint64(time.Now().UnixNano())
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&database.Job{}).Where("target = ? AND kind = ? AND state = ?", node.UUID, "user-sync", "failed").Updates(map[string]interface{}{"state": "pending", "attempts": 0, "available_at": time.Now().UTC()}).Error; err != nil {
			return err
		}
		for _, user := range users {
			if err := jobs.Enqueue(tx, "user-sync", strconv.FormatUint(user.ID, 10), node.UUID, revision, jobs.UserSync{ID: user.ID, VlessUUID: user.VlessUUID}); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		log.Printf("[NODES] schedule reconciliation: %v", err)
	}
}

func (s *Service) SyncUserToNodes(user *database.User, previous *string) {
	if user == nil {
		return
	}
	if err := jobs.EnqueueUser(s.db, user, previous, false); err != nil {
		log.Printf("[NODES] enqueue user: %v", err)
	}
}
func (s *Service) RemoveUserFromNodes(user *database.User) {
	if user == nil {
		return
	}
	if err := jobs.EnqueueUser(s.db, user, nil, true); err != nil {
		log.Printf("[NODES] enqueue removal: %v", err)
	}
}
func (s *Service) SyncAllUsersToConnectedNodes() {
	var targets []database.Node
	if err := s.db.Where("is_disabled = ? AND is_connected = ?", false, true).Find(&targets).Error; err != nil {
		return
	}
	for i := range targets {
		s.SyncAllUsersToNode(&targets[i])
	}
}

func (s *Service) ApplyUserJob(ctx context.Context, job *database.Job) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	var node database.Node
	if err := s.db.Where("uuid = ?", job.Target).First(&node).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	if node.IsDisabled {
		return nil
	}
	if !node.IsConnected {
		return errors.New("node offline")
	}
	var payload jobs.UserSync
	if err := json.Unmarshal([]byte(job.Payload), &payload); err != nil {
		return err
	}
	var user database.User
	err := s.db.Where("id = ?", payload.ID).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		client := s.Client()
		if client == nil {
			return errors.New("node client unavailable")
		}
		return client.RemoveUser(&node, RemoveUserRequestPayload{Username: strconv.FormatUint(payload.ID, 10), HashData: NodeUserHashData{VlessUUID: payload.VlessUUID}})
	}
	if err != nil {
		return err
	}
	inbounds, err := s.GetUserResolvedInbounds(user.ID)
	if err != nil {
		return err
	}
	var previous *string
	if payload.PreviousUUID != "" {
		previous = &payload.PreviousUUID
	}
	return s.syncUserToSingleNode(&node, &user, previous, inbounds)
}

// ApplyRetireJob stops Xray on a deleted node using the job's snapshot.
func (s *Service) ApplyRetireJob(ctx context.Context, job *database.Job) error {
	var node database.Node
	if err := json.Unmarshal([]byte(job.Payload), &node); err != nil {
		return err
	}
	client := s.Client()
	if client == nil {
		return errors.New("node client unavailable")
	}
	ok, err := client.StopXray(&node)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("node rejected stop")
	}
	return nil
}

func (s *Service) ApplyNodeJob(ctx context.Context, job *database.Job) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	node, err := s.GetByUUID(job.Target)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if job.Kind == "node-stop" {
		if !node.IsDisabled {
			return nil
		}
		client := s.Client()
		if client == nil {
			return errors.New("node client unavailable")
		}
		ok, err := client.StopXray(node)
		if err != nil {
			return err
		}
		if !ok {
			return errors.New("node rejected stop")
		}
		return nil
	}
	if node.IsDisabled {
		return nil
	}
	ok, err := s.StartNode(node, true)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("node rejected start")
	}
	return nil
}

func (s *Service) SetDisabled(nodeUUID string, disabled bool) (*database.Node, error) {
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var node database.Node
		if err := tx.Where("uuid = ?", nodeUUID).First(&node).Error; err != nil {
			return err
		}
		updates := map[string]interface{}{"is_disabled": disabled, "is_connecting": false, "updated_at": time.Now().UTC()}
		if disabled {
			updates["is_connected"] = false
			updates["last_status_change"] = time.Now().UTC()
		}
		if err := tx.Model(&database.Node{}).Where("uuid = ?", nodeUUID).Updates(updates).Error; err != nil {
			return err
		}
		kind, event := "node-start", "node.enabled"
		if disabled {
			kind, event = "node-stop", "node.disabled"
		}
		if err := s.emitNodeByUUID(tx, nodeUUID, event); err != nil {
			return err
		}
		return jobs.EnqueueNode(tx, nodeUUID, kind)
	})
	if err != nil {
		return nil, err
	}
	return s.GetByUUID(nodeUUID)
}

func logf(format string, args ...interface{}) { log.Printf(format, args...) }
