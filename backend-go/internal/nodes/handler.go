package nodes

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"gowave/internal/configprofiles"
	"gowave/internal/database"
	"gowave/internal/httpx"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

type UpdateNodeDTO struct {
	UUID                      string   `json:"uuid"`
	Name                      *string  `json:"name"`
	Address                   *string  `json:"address"`
	Port                      *int     `json:"port"`
	ProxyURL                  *string  `json:"proxyUrl"`
	IsTrafficTrackingActive   *bool    `json:"isTrafficTrackingActive"`
	TrafficLimitBytes         *uint64  `json:"trafficLimitBytes"`
	NotifyPercent             *int     `json:"notifyPercent"`
	TrafficResetDay           *int     `json:"trafficResetDay"`
	CountryCode               *string  `json:"countryCode"`
	ConsumptionMultiplier     *float64 `json:"consumptionMultiplier"`
	NodeConsumptionMultiplier *float64 `json:"nodeConsumptionMultiplier"`
	ConfigProfile             *struct {
		ActiveConfigProfileUUID *string  `json:"activeConfigProfileUuid"`
		ActiveInbounds          []string `json:"activeInbounds"`
	} `json:"configProfile"`
	ProviderUUID     *string   `json:"providerUuid"`
	Tags             *[]string `json:"tags"`
	ActivePluginUUID *string   `json:"activePluginUuid"`
	IntegrationUUIDs *[]string `json:"integrationUuids"`
	Note             *string   `json:"note"`
	IPs              *[]NodeIP `json:"ips"`
	// TrafficTrackingStart is a Gowave extension: how the counter starts when
	// this update enables traffic tracking (keep, zero or period).
	TrafficTrackingStart *string `json:"trafficTrackingStart"`
}

func formatNodeResponse(service *Service, n *database.Node) map[string]interface{} {
	return formatNodeWith(service.DB(), service, n)
}

// formatNodeWith formats a node reading related rows through db, which may be
// an open transaction.
func formatNodeWith(db *gorm.DB, service *Service, n *database.Node) map[string]interface{} {
	return formatNodeWithInbounds(service, n, loadActiveInbounds(db, []database.Node{*n})[n.UUID])
}

// loadActiveInbounds returns each node's active inbounds: the inbounds linked
// to the node, or every inbound of its active profile when it has no links.
// It issues at most two queries for any number of nodes.
func loadActiveInbounds(db *gorm.DB, nodes []database.Node) map[string][]database.ConfigProfileInbound {
	result := make(map[string][]database.ConfigProfileInbound, len(nodes))
	var nodeUUIDs []string
	for _, n := range nodes {
		if n.ActiveConfigProfileUUID != nil && *n.ActiveConfigProfileUUID != "" {
			nodeUUIDs = append(nodeUUIDs, n.UUID)
		}
	}
	if len(nodeUUIDs) == 0 {
		return result
	}

	var linked []struct {
		database.ConfigProfileInbound
		LinkedNodeUUID string `gorm:"column:linked_node_uuid"`
	}
	db.Table("config_profile_inbounds").
		Select("config_profile_inbounds.*, config_profile_inbounds_to_nodes.node_uuid AS linked_node_uuid").
		Joins("JOIN config_profile_inbounds_to_nodes ON config_profile_inbounds_to_nodes.config_profile_inbound_uuid = config_profile_inbounds.uuid").
		Where("config_profile_inbounds_to_nodes.node_uuid IN ?", nodeUUIDs).
		Find(&linked)
	for _, row := range linked {
		result[row.LinkedNodeUUID] = append(result[row.LinkedNodeUUID], row.ConfigProfileInbound)
	}

	profileNodes := make(map[string][]string)
	var profileUUIDs []string
	for _, n := range nodes {
		if n.ActiveConfigProfileUUID == nil || *n.ActiveConfigProfileUUID == "" || len(result[n.UUID]) > 0 {
			continue
		}
		profile := *n.ActiveConfigProfileUUID
		if _, seen := profileNodes[profile]; !seen {
			profileUUIDs = append(profileUUIDs, profile)
		}
		profileNodes[profile] = append(profileNodes[profile], n.UUID)
	}
	if len(profileUUIDs) == 0 {
		return result
	}
	var profileInbounds []database.ConfigProfileInbound
	db.Where("profile_uuid IN ?", profileUUIDs).Find(&profileInbounds)
	for _, ib := range profileInbounds {
		for _, nodeUUID := range profileNodes[ib.ProfileUUID] {
			result[nodeUUID] = append(result[nodeUUID], ib)
		}
	}
	return result
}

func formatNodeWithInbounds(service *Service, n *database.Node, inbounds []database.ConfigProfileInbound) map[string]interface{} {
	tags := []string(n.Tags)
	if tags == nil {
		tags = []string{}
	}

	integrationUuids := []string(n.IntegrationUUIDs)
	if integrationUuids == nil {
		integrationUuids = []string{}
	}

	ips := decodeNodeIPs(n.IPs)

	var lastStatusChange *string
	if n.LastStatusChange != nil {
		s := n.LastStatusChange.UTC().Format(time.RFC3339)
		lastStatusChange = &s
	}

	configProfileObj := map[string]interface{}{
		"activeConfigProfileUuid": n.ActiveConfigProfileUUID,
		"activeInbounds":          []interface{}{},
	}
	if n.ActiveConfigProfileUUID != nil && *n.ActiveConfigProfileUUID != "" {
		activeIbList := make([]map[string]interface{}, 0, len(inbounds))
		for _, ib := range inbounds {
			var raw json.RawMessage = []byte("{}")
			if ib.RawInbound != "" {
				if sorted, err := configprofiles.SortInboundConfig([]byte(ib.RawInbound)); err == nil {
					raw = json.RawMessage(sorted)
				} else {
					raw = json.RawMessage(ib.RawInbound)
				}
			}
			activeIbList = append(activeIbList, map[string]interface{}{
				"uuid":        ib.UUID,
				"profileUuid": ib.ProfileUUID,
				"tag":         ib.Tag,
				"type":        ib.Type,
				"network":     ib.Network,
				"security":    ib.Security,
				"port":        ib.Port,
				"rawInbound":  raw,
			})
		}
		configProfileObj["activeInbounds"] = activeIbList
	}

	cm := NanoToMultiplier(n.ConsumptionMultiplier)
	ncm := NanoToMultiplier(n.NodeConsumptionMultiplier)
	vp := n.ViewPosition
	if vp == 0 {
		vp = 1
	}
	trd := n.TrafficResetDay
	if trd == 0 {
		trd = 1
	}
	np := n.NotifyPercent
	if np == 0 {
		np = 80
	}

	statusMsg := n.LastStatusMessage
	if statusMsg != nil && (*statusMsg == "OK" || *statusMsg == "") {
		statusMsg = nil
	}
	// A connected node with a status message is serving its previous config
	// because it rejected the latest one.
	if statusMsg == nil && n.IsConnected && !n.IsDisabled && n.XrayConfigError != nil && *n.XrayConfigError != "" {
		statusMsg = n.XrayConfigError
	}

	var systemObj interface{} = nil
	var versionsObj interface{} = nil
	xrayUptime := int64(0)
	usersOnline := 0

	metrics := service.GetNodeMetrics(n.UUID)
	if metrics != nil {
		usersOnline = metrics.OnlineUsers
		xrayUptime = metrics.XrayUptime
		if metrics.Versions != nil {
			versionsObj = metrics.Versions
		}
		if metrics.System != nil && metrics.System.System != nil && metrics.System.System.Stats != nil {
			st := metrics.System.System.Stats
			info := metrics.System.System.Info

			memTotal := uint64(0)
			if info != nil && info.MemoryTotal > 0 {
				memTotal = info.MemoryTotal
			} else if st.MemoryFree > 0 || st.MemoryUsed > 0 {
				memTotal = st.MemoryFree + st.MemoryUsed
			}

			arch := "x64"
			cpus := 2
			cpuModel := "CPU"
			hostname := n.Name
			platform := "linux"
			release := "linux"
			hType := "Linux"
			version := "1.0"
			ifaces := []string{"eth0"}

			if info != nil {
				if info.Arch != "" {
					arch = info.Arch
				}
				if info.CPUs > 0 {
					cpus = info.CPUs
				}
				if info.CPUModel != "" {
					cpuModel = info.CPUModel
				}
				if info.Hostname != "" {
					hostname = info.Hostname
				}
				if info.Platform != "" {
					platform = info.Platform
				}
				if info.Release != "" {
					release = info.Release
				}
				if info.Type != "" {
					hType = info.Type
				}
				if info.Version != "" {
					version = info.Version
				}
				if len(info.NetworkInterfaces) > 0 {
					ifaces = info.NetworkInterfaces
				}
			}

			systemObj = map[string]interface{}{
				"info": map[string]interface{}{
					"arch":              arch,
					"cpus":              cpus,
					"cpuModel":          cpuModel,
					"memoryTotal":       memTotal,
					"hostname":          hostname,
					"platform":          platform,
					"release":           release,
					"type":              hType,
					"version":           version,
					"networkInterfaces": ifaces,
				},
				"stats": map[string]interface{}{
					"memoryFree": st.MemoryFree,
					"memoryUsed": st.MemoryUsed,
					"uptime":     uint64(st.Uptime),
					"loadAvg":    st.LoadAvg,
					"interface":  st.Interface,
				},
			}
		}
	}

	return map[string]interface{}{
		"uuid":                      n.UUID,
		"id":                        n.ID,
		"name":                      n.Name,
		"address":                   n.Address,
		"port":                      n.Port,
		"proxyUrl":                  n.ProxyURL,
		"isConnected":               n.IsConnected,
		"isDisabled":                n.IsDisabled,
		"isConnecting":              n.IsConnecting,
		"lastStatusChange":          lastStatusChange,
		"lastStatusMessage":         statusMsg,
		"isTrafficTrackingActive":   n.IsTrafficTrackingActive,
		"trafficResetDay":           trd,
		"trafficLimitBytes":         n.TrafficLimitBytes,
		"trafficUsedBytes":          n.TrafficUsedBytes,
		"notifyPercent":             np,
		"viewPosition":              vp,
		"countryCode":               n.CountryCode,
		"consumptionMultiplier":     cm,
		"nodeConsumptionMultiplier": ncm,
		"tags":                      tags,
		"integrationUuids":          integrationUuids,
		"ips":                       ips,
		"createdAt":                 n.CreatedAt.UTC().Format(time.RFC3339),
		"updatedAt":                 n.UpdatedAt.UTC().Format(time.RFC3339),
		"configProfile":             configProfileObj,
		"providerUuid":              n.ProviderUUID,
		"provider":                  nil,
		"activePluginUuid":          n.ActivePluginUUID,
		"system":                    systemObj,
		"versions":                  versionsObj,
		"xrayUptime":                xrayUptime,
		"usersOnline":               usersOnline,
		"note":                      n.Note,
	}
}

func (h *Handler) GetNodes(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	nodesList, err := h.service.GetAll()
	if err != nil {
		http.Error(w, `{"message":"Failed to fetch nodes"}`, http.StatusInternalServerError)
		return
	}

	// Health is refreshed by StartHealthCheckLoop and on node creation; a
	// read must not fan out node RPCs and status writes on every poll.
	inbounds := loadActiveInbounds(h.service.DB(), nodesList)
	resp := make([]map[string]interface{}, 0, len(nodesList))
	for i := range nodesList {
		resp = append(resp, formatNodeWithInbounds(h.service, &nodesList[i], inbounds[nodesList[i].UUID]))
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": resp,
	})
}

func (h *Handler) GetTags(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	tags := []string{}
	nodesList, _ := h.service.GetAll()
	tagSet := make(map[string]bool)
	for _, n := range nodesList {
		for _, t := range n.Tags {
			if t != "" && !tagSet[t] {
				tagSet[t] = true
				tags = append(tags, t)
			}
		}
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": map[string]interface{}{
			"tags": tags,
		},
	})
}

func (h *Handler) CreateNode(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var dto CreateNodeDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		http.Error(w, `{"message":"Invalid request body"}`, http.StatusBadRequest)
		return
	}

	node, err := h.service.Create(dto)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": err.Error()})
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": formatNodeResponse(h.service, node),
	})
}

func (h *Handler) GetNode(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	uuid := chi.URLParam(r, "uuid")
	node, err := h.service.GetByUUID(uuid)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Node not found"})
		return
	}

	metrics := h.service.GetNodeMetrics(node.UUID)
	if (metrics == nil || metrics.Versions == nil) && !node.IsDisabled {
		h.service.CheckNodeHealth(node)
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": formatNodeResponse(h.service, node),
	})
}

func (h *Handler) handleUpdateNode(w http.ResponseWriter, r *http.Request, targetUUID string) {
	w.Header().Set("Content-Type", "application/json")

	var dto UpdateNodeDTO
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		http.Error(w, `{"message":"Invalid request body"}`, http.StatusBadRequest)
		return
	}

	if targetUUID == "" {
		targetUUID = dto.UUID
	}
	if targetUUID == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Node UUID is required"})
		return
	}

	_, err := h.service.GetByUUID(targetUUID)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Node not found"})
		return
	}

	updates := map[string]interface{}{}
	if dto.Name != nil {
		updates["name"] = strings.TrimSpace(*dto.Name)
	}
	if dto.Address != nil {
		updates["address"] = strings.TrimSpace(*dto.Address)
	}
	if dto.Port != nil {
		updates["port"] = *dto.Port
	}
	if dto.ProxyURL != nil {
		updates["proxy_url"] = dto.ProxyURL
	}
	if dto.IsTrafficTrackingActive != nil {
		updates["is_traffic_tracking_active"] = *dto.IsTrafficTrackingActive
	}
	if dto.TrafficLimitBytes != nil {
		updates["traffic_limit_bytes"] = *dto.TrafficLimitBytes
	}
	if dto.NotifyPercent != nil {
		updates["notify_percent"] = *dto.NotifyPercent
	}
	if dto.TrafficResetDay != nil {
		updates["traffic_reset_day"] = *dto.TrafficResetDay
	}
	if dto.CountryCode != nil {
		updates["country_code"] = *dto.CountryCode
	}
	if dto.ConsumptionMultiplier != nil {
		updates["consumption_multiplier"] = MultiplierToNano(*dto.ConsumptionMultiplier)
	}
	if dto.NodeConsumptionMultiplier != nil {
		updates["node_consumption_multiplier"] = MultiplierToNano(*dto.NodeConsumptionMultiplier)
	}
	if dto.ProviderUUID != nil {
		updates["provider_uuid"] = dto.ProviderUUID
	}
	if dto.ActivePluginUUID != nil {
		updates["active_plugin_uuid"] = dto.ActivePluginUUID
	}
	if dto.Note != nil {
		updates["note"] = dto.Note
	}
	if dto.Tags != nil {
		updates["tags"] = database.StringArray(*dto.Tags)
	}
	if dto.IntegrationUUIDs != nil {
		updates["integration_uuids"] = database.StringArray(*dto.IntegrationUUIDs)
	}
	if dto.IPs != nil {
		encoded, err := encodeNodeIPs(*dto.IPs)
		if err != nil {
			httpx.Error(w, r, http.StatusBadRequest, "E000", err.Error())
			return
		}
		updates["ips"] = encoded
	}

	trackingStart := ""
	if dto.TrafficTrackingStart != nil {
		if !ValidTrackingStart(*dto.TrafficTrackingStart) {
			httpx.Error(w, r, http.StatusBadRequest, "E000", "trafficTrackingStart must be keep, zero or period")
			return
		}
		trackingStart = *dto.TrafficTrackingStart
	}

	var profileUUID *string
	var activeInbounds []string
	profileUpdate := dto.ConfigProfile != nil
	if profileUpdate {
		profileUUID = dto.ConfigProfile.ActiveConfigProfileUUID
		activeInbounds = dto.ConfigProfile.ActiveInbounds
	}
	node, err := h.service.UpdateWithConfigProfile(targetUUID, updates, profileUpdate, profileUUID, activeInbounds, trackingStart)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Failed to update node"})
		return
	}

	if !node.IsDisabled && (dto.ConfigProfile != nil || dto.Address != nil || dto.Port != nil || dto.ProxyURL != nil || dto.IntegrationUUIDs != nil) {
		go h.service.StartNode(node, dto.IntegrationUUIDs != nil)
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": formatNodeResponse(h.service, node),
	})
}

func (h *Handler) UpdateNode(w http.ResponseWriter, r *http.Request) {
	uuid := chi.URLParam(r, "uuid")
	h.handleUpdateNode(w, r, uuid)
}

func (h *Handler) PatchNode(w http.ResponseWriter, r *http.Request) {
	uuid := chi.URLParam(r, "uuid")
	h.handleUpdateNode(w, r, uuid)
}

func (h *Handler) DeleteNode(w http.ResponseWriter, r *http.Request) {
	uuid := chi.URLParam(r, "uuid")
	if err := h.service.Delete(uuid); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Failed to delete node"})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) EnableNode(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	uuidParam := chi.URLParam(r, "uuid")
	node, err := h.service.SetDisabled(uuidParam, false)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Node not found"})
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": formatNodeResponse(h.service, node),
	})
}

func (h *Handler) DisableNode(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	uuidParam := chi.URLParam(r, "uuid")
	node, err := h.service.SetDisabled(uuidParam, true)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Node not found"})
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": formatNodeResponse(h.service, node),
	})
}

// GetTrafficSummary reports each node's lifetime traffic, which the monthly
// trafficUsedBytes reset does not clear. Gowave extension, not in the contract.
func (h *Handler) GetTrafficSummary(w http.ResponseWriter, r *http.Request) {
	totals, err := h.service.LifetimeTraffic()
	if err != nil {
		httpx.Error(w, r, http.StatusInternalServerError, "E000", "Failed to read node traffic")
		return
	}
	nodes := make([]map[string]interface{}, 0, len(totals))
	for nodeUUID, total := range totals {
		nodes = append(nodes, map[string]interface{}{"uuid": nodeUUID, "lifetimeTrafficBytes": total})
	}
	httpx.Response(w, map[string]interface{}{"nodes": nodes})
}

func (h *Handler) ResetTraffic(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	uuidParam := chi.URLParam(r, "uuid")
	node, err := h.service.Update(uuidParam, map[string]interface{}{"traffic_used_bytes": 0})
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Node not found"})
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": formatNodeResponse(h.service, node),
	})
}

func (h *Handler) RestartNode(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	uuidParam := chi.URLParam(r, "uuid")
	node, err := h.service.GetByUUID(uuidParam)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Node not found"})
		return
	}

	ok, startErr := h.service.StartNode(node, true)
	if !ok || startErr != nil {
		msg := "Failed to restart node"
		if startErr != nil {
			msg = startErr.Error()
		}
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message": msg,
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": map[string]interface{}{
			"success": true,
		},
	})
}

func (h *Handler) RestartAllNodes(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	nodes, _ := h.service.GetAll()
	for _, n := range nodes {
		go h.service.StartNode(&n, true)
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": map[string]interface{}{
			"success": true,
		},
	})
}

func (h *Handler) ReorderNodes(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var body struct {
		Nodes []struct {
			UUID         string `json:"uuid"`
			ViewPosition int    `json:"viewPosition"`
		} `json:"nodes"`
		Items []struct {
			UUID         string `json:"uuid"`
			ViewPosition int    `json:"viewPosition"`
		} `json:"items"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	items := body.Nodes
	if len(items) == 0 {
		items = body.Items
	}

	for _, item := range items {
		if item.UUID != "" {
			h.service.DB().Model(&database.Node{}).Where("uuid = ?", item.UUID).Update("view_position", item.ViewPosition)
		}
	}

	h.GetNodes(w, r)
}

func (h *Handler) BulkActions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if strings.HasSuffix(r.URL.Path, "profile-modification") {
		var body struct {
			UUIDs         []string `json:"uuids"`
			ConfigProfile struct {
				ActiveConfigProfileUUID string   `json:"activeConfigProfileUuid"`
				ActiveInbounds          []string `json:"activeInbounds"`
			} `json:"configProfile"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, `{"message":"Invalid request body"}`, http.StatusBadRequest)
			return
		}

		var profileUUID *string
		if body.ConfigProfile.ActiveConfigProfileUUID != "" {
			profileUUID = &body.ConfigProfile.ActiveConfigProfileUUID
		}
		if err := h.service.BulkUpdateConfigProfile(body.UUIDs, profileUUID, body.ConfigProfile.ActiveInbounds); err != nil {
			http.Error(w, `{"message":"Failed to update node config profiles"}`, http.StatusBadRequest)
			return
		}
		for _, nodeUUID := range body.UUIDs {
			if node, err := h.service.GetByUUID(nodeUUID); err == nil && !node.IsDisabled {
				go h.service.StartNode(node, false)
			}
		}

		w.WriteHeader(http.StatusNoContent)
		return
	}

	if strings.HasSuffix(r.URL.Path, "bulk-actions/update") {
		var body struct {
			UUIDs  []string               `json:"uuids"`
			Fields map[string]interface{} `json:"fields"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, `{"message":"Invalid request body"}`, http.StatusBadRequest)
			return
		}
		if err := h.service.BulkUpdate(body.UUIDs, body.Fields); err != nil {
			http.Error(w, err.Error(), 400)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}

	var body struct {
		UUIDs  []string `json:"uuids"`
		Action string   `json:"action"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"message":"Invalid request body"}`, http.StatusBadRequest)
		return
	}

	for _, nodeUuid := range body.UUIDs {
		switch strings.ToUpper(body.Action) {
		case "DISABLE":
			_, _ = h.service.SetDisabled(nodeUuid, true)
		case "ENABLE":
			_, _ = h.service.SetDisabled(nodeUuid, false)
		case "RESET_TRAFFIC":
			_, _ = h.service.Update(nodeUuid, map[string]interface{}{"traffic_used_bytes": 0})
		case "RESTART":
			if node, err := h.service.GetByUUID(nodeUuid); err == nil {
				go h.service.StartNode(node, true)
			}
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) GetNodeLogs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	nodeUUID := chi.URLParam(r, "uuid")
	logType := r.URL.Query().Get("type")
	if logType == "" {
		logType = "xray"
	}
	linesStr := r.URL.Query().Get("lines")
	lines := 500
	if l, err := strconv.Atoi(linesStr); err == nil && l > 0 {
		lines = l
	}

	node, err := h.service.GetByUUID(nodeUUID)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"message": "Node not found"})
		return
	}

	client := h.service.Client()
	if client == nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"message": "Node client not configured"})
		return
	}

	logs, err := client.GetLogs(node, logType, lines)
	if err != nil {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"response": map[string]interface{}{
				"logs": []string{fmt.Sprintf("[ERROR] Failed to fetch logs from node: %v", err)},
			},
		})
		return
	}

	if logs == nil {
		logs = []string{}
	}

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"response": map[string]interface{}{
			"logs": logs,
		},
	})
}

func (h *Handler) GetNodeUpdates(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	nodeUUID := chi.URLParam(r, "uuid")
	repo := r.URL.Query().Get("repo")

	node, err := h.service.GetByUUID(nodeUUID)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"message": "Node not found"})
		return
	}

	client := h.service.Client()
	if client == nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"message": "Node client not configured"})
		return
	}

	updates, err := client.CheckUpdates(node, repo)
	if err != nil {
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"message": err.Error()})
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"response": updates,
	})
}

func (h *Handler) ApplyNodeUpdate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	nodeUUID := chi.URLParam(r, "uuid")

	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"message": "invalid body"})
		return
	}

	node, err := h.service.GetByUUID(nodeUUID)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"message": "Node not found"})
		return
	}

	client := h.service.Client()
	if client == nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"message": "Node client not configured"})
		return
	}

	result, err := client.ApplyUpdate(node, body)
	if err != nil {
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"message": err.Error()})
		return
	}

	go func(n database.Node) {
		time.Sleep(2 * time.Second)
		h.service.CheckNodeHealth(&n)
	}(*node)

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"response": result,
	})
}
