package nodes

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"remnawave-go/internal/configprofiles"
	"remnawave-go/internal/database"

	"github.com/go-chi/chi/v5"
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
	IPs              *[]string `json:"ips"`
}

func formatNodeResponse(service *Service, n *database.Node) map[string]interface{} {
	db := service.DB()
	tags := []string(n.Tags)
	if tags == nil {
		tags = []string{}
	}

	integrationUuids := []string(n.IntegrationUUIDs)
	if integrationUuids == nil {
		integrationUuids = []string{}
	}

	ips := []string{}
	if n.IPs != "" {
		_ = json.Unmarshal([]byte(n.IPs), &ips)
	}
	if ips == nil {
		ips = []string{}
	}

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
		var inbounds []database.ConfigProfileInbound
		db.Table("config_profile_inbounds").
			Joins("JOIN config_profile_inbounds_to_nodes ON config_profile_inbounds_to_nodes.config_profile_inbound_uuid = config_profile_inbounds.uuid").
			Where("config_profile_inbounds_to_nodes.node_uuid = ?", n.UUID).
			Find(&inbounds)
		if len(inbounds) == 0 {
			db.Where("profile_uuid = ?", *n.ActiveConfigProfileUUID).Find(&inbounds)
		}
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

	resp := make([]map[string]interface{}, 0, len(nodesList))
	for _, n := range nodesList {
		resp = append(resp, formatNodeResponse(h.service, &n))
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
		b, _ := json.Marshal(*dto.IPs)
		updates["ips"] = string(b)
	}

	var profileUUID *string
	var activeInbounds []string
	profileUpdate := dto.ConfigProfile != nil
	if profileUpdate {
		profileUUID = dto.ConfigProfile.ActiveConfigProfileUUID
		activeInbounds = dto.ConfigProfile.ActiveInbounds
	}
	node, err := h.service.UpdateWithConfigProfile(targetUUID, updates, profileUpdate, profileUUID, activeInbounds)
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
	node, err := h.service.Update(uuidParam, map[string]interface{}{"is_disabled": false})
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
	node, err := h.service.Update(uuidParam, map[string]interface{}{"is_disabled": true})
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Node not found"})
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": formatNodeResponse(h.service, node),
	})
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
	if node, err := h.service.GetByUUID(uuidParam); err == nil {
		go h.service.StartNode(node, true)
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
		for _, nodeUuid := range body.UUIDs {
			_, _ = h.service.Update(nodeUuid, body.Fields)
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
		switch body.Action {
		case "disable":
			_, _ = h.service.Update(nodeUuid, map[string]interface{}{"is_disabled": true})
		case "enable":
			_, _ = h.service.Update(nodeUuid, map[string]interface{}{"is_disabled": false})
		case "resetTraffic":
			_, _ = h.service.Update(nodeUuid, map[string]interface{}{"traffic_used_bytes": 0})
		case "restart":
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

	if h.service.client == nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"message": "Node client not configured"})
		return
	}

	logs, err := h.service.client.GetLogs(node, logType, lines)
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

	if h.service.client == nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"message": "Node client not configured"})
		return
	}

	updates, err := h.service.client.CheckUpdates(node, repo)
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

	if h.service.client == nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"message": "Node client not configured"})
		return
	}

	result, err := h.service.client.ApplyUpdate(node, body)
	if err != nil {
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"message": err.Error()})
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"response": result,
	})
}
