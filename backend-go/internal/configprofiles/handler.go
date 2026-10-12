package configprofiles

import (
	"github.com/google/uuid"

	"encoding/json"
	"net/http"
	"strings"
	"time"

	"remnawave-go/internal/database"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

type InboundResponse struct {
	UUID         string          `json:"uuid"`
	ProfileUUID  string          `json:"profileUuid"`
	Tag          string          `json:"tag"`
	Type         string          `json:"type"`
	Network      *string         `json:"network"`
	Security     *string         `json:"security"`
	Port         *int            `json:"port"`
	RawInbound   json.RawMessage `json:"rawInbound"`
	ActiveSquads []string        `json:"activeSquads,omitempty"`
}

type NodeSummary struct {
	UUID        string `json:"uuid"`
	Name        string `json:"name"`
	CountryCode string `json:"countryCode"`
}

type ProfileResponse struct {
	UUID         string            `json:"uuid"`
	ViewPosition int               `json:"viewPosition"`
	Name         string            `json:"name"`
	Tags         []string          `json:"tags"`
	Config       json.RawMessage   `json:"config"`
	Inbounds     []InboundResponse `json:"inbounds"`
	Nodes        []NodeSummary     `json:"nodes"`
	CreatedAt    string            `json:"createdAt"`
	UpdatedAt    string            `json:"updatedAt"`
}

func formatInbound(ib *database.ConfigProfileInbound) InboundResponse {
	raw := json.RawMessage("{}")
	if ib.RawInbound != "" {
		if sorted, err := SortInboundConfig([]byte(ib.RawInbound)); err == nil {
			raw = json.RawMessage(sorted)
		} else {
			raw = json.RawMessage(ib.RawInbound)
		}
	}
	return InboundResponse{
		UUID:         ib.UUID,
		ProfileUUID:  ib.ProfileUUID,
		Tag:          ib.Tag,
		Type:         ib.Type,
		Network:      ib.Network,
		Security:     ib.Security,
		Port:         ib.Port,
		RawInbound:   raw,
		ActiveSquads: []string{},
	}
}
func (h *Handler) formatProfile(p *database.ConfigProfile) ProfileResponse {
	cfg := json.RawMessage("{}")
	if p.Config != "" {
		if sorted, err := SortXrayConfig([]byte(p.Config)); err == nil {
			cfg = json.RawMessage(sorted)
		} else {
			cfg = json.RawMessage(p.Config)
		}
	}

	tags := []string(p.Tags)
	if tags == nil {
		tags = []string{}
	}

	inboundsDB, _ := h.service.GetInboundsByProfileUUID(p.UUID)
	inboundsResp := make([]InboundResponse, 0, len(inboundsDB))
	for _, ib := range inboundsDB {
		inboundsResp = append(inboundsResp, formatInbound(&ib))
	}

	var nodes []database.Node
	h.service.DB().Where("active_config_profile_uuid = ?", p.UUID).Find(&nodes)
	nodesResp := make([]NodeSummary, 0, len(nodes))
	for _, n := range nodes {
		nodesResp = append(nodesResp, NodeSummary{
			UUID:        n.UUID,
			Name:        n.Name,
			CountryCode: n.CountryCode,
		})
	}

	pos := p.ViewPosition
	if pos == 0 {
		pos = 1
	}

	return ProfileResponse{
		UUID:         p.UUID,
		ViewPosition: pos,
		Name:         p.Name,
		Tags:         tags,
		Config:       cfg,
		Inbounds:     inboundsResp,
		Nodes:        nodesResp,
		CreatedAt:    p.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    p.UpdatedAt.Format(time.RFC3339),
	}
}
func (h *Handler) GetConfigProfilesTags(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	tags := []string{}
	profiles, _ := h.service.GetAllProfiles()
	tagSet := make(map[string]bool)
	for _, p := range profiles {
		for _, t := range p.Tags {
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

func (h *Handler) SetConfigProfilesTags(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var body struct {
		UUID string   `json:"uuid"`
		Tags []string `json:"tags"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"message":"Invalid request body"}`, http.StatusBadRequest)
		return
	}

	if body.Tags == nil {
		body.Tags = []string{}
	}

	if err := h.service.SetProfileTags(body.UUID, body.Tags); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": err.Error()})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": map[string]interface{}{
			"uuid": body.UUID,
			"tags": body.Tags,
		},
	})
}

func (h *Handler) GetProfiles(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	profiles, err := h.service.GetAllProfiles()
	if err != nil {
		http.Error(w, `{"message":"Failed to fetch config profiles"}`, http.StatusInternalServerError)
		return
	}

	respProfiles := make([]ProfileResponse, 0, len(profiles))
	for _, p := range profiles {
		respProfiles = append(respProfiles, h.formatProfile(&p))
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": map[string]interface{}{
			"total":          len(respProfiles),
			"configProfiles": respProfiles,
		},
	})
}

func (h *Handler) CreateConfigProfile(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var body struct {
		Name   string          `json:"name"`
		Config json.RawMessage `json:"config"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"message":"Invalid request body"}`, http.StatusBadRequest)
		return
	}

	if body.Name == "" || len(body.Config) == 0 {
		http.Error(w, `{"message":"Name and config are required"}`, http.StatusBadRequest)
		return
	}

	profile, err := h.service.CreateProfile(body.Name, body.Config)
	if err != nil {
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": err.Error()})
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": h.formatProfile(profile),
	})
}
func (h *Handler) GetConfigProfile(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	uuidParam := chi.URLParam(r, "uuid")
	profile, err := h.service.GetProfileByUUID(uuidParam)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Config profile not found"})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": h.formatProfile(profile),
	})
}

func (h *Handler) UpdateConfigProfile(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var body struct {
		UUID   string           `json:"uuid"`
		Name   *string          `json:"name"`
		Config *json.RawMessage `json:"config"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"message":"Invalid request body"}`, http.StatusBadRequest)
		return
	}

	if body.UUID == "" {
		http.Error(w, `{"message":"UUID is required"}`, http.StatusBadRequest)
		return
	}

	var cfg []byte
	if body.Config != nil {
		cfg = *body.Config
	}

	profile, err := h.service.UpdateProfile(body.UUID, body.Name, cfg)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": err.Error()})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": h.formatProfile(profile),
	})
}
func (h *Handler) DeleteConfigProfile(w http.ResponseWriter, r *http.Request) {
	uuidParam := chi.URLParam(r, "uuid")
	_ = h.service.DeleteProfile(uuidParam)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ReorderConfigProfiles(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var body struct {
		Items []struct {
			UUID         string `json:"uuid"`
			ViewPosition int    `json:"viewPosition"`
		} `json:"items"`
		ConfigProfiles []struct {
			UUID         string `json:"uuid"`
			ViewPosition int    `json:"viewPosition"`
		} `json:"configProfiles"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	items := body.Items
	if len(items) == 0 {
		items = body.ConfigProfiles
	}

	for _, item := range items {
		if item.UUID != "" {
			h.service.DB().Model(&database.ConfigProfile{}).Where("uuid = ?", item.UUID).Update("view_position", item.ViewPosition)
		}
	}

	h.GetProfiles(w, r)
}

func (h *Handler) GetProfileInbounds(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	uuidParam := chi.URLParam(r, "uuid")
	inbounds, err := h.service.GetInboundsByProfileUUID(uuidParam)
	if err != nil {
		inbounds = []database.ConfigProfileInbound{}
	}

	resp := make([]InboundResponse, 0, len(inbounds))
	for _, ib := range inbounds {
		resp = append(resp, formatInbound(&ib))
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": map[string]interface{}{
			"total":    len(resp),
			"inbounds": resp,
		},
	})
}

func (h *Handler) GetAllInbounds(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	inbounds, err := h.service.GetAllInbounds()
	if err != nil {
		inbounds = []database.ConfigProfileInbound{}
	}

	resp := make([]InboundResponse, 0, len(inbounds))
	for _, ib := range inbounds {
		resp = append(resp, formatInbound(&ib))
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": map[string]interface{}{
			"total":    len(resp),
			"inbounds": resp,
		},
	})
}

func (h *Handler) GetComputedConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	uuidParam := chi.URLParam(r, "uuid")
	profile, err := h.service.GetProfileByUUID(uuidParam)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Config profile not found"})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": h.formatProfile(profile),
	})
}

func (h *Handler) GetHostsTags(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": map[string]interface{}{
			"tags": []string{},
		},
	})
}

func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func nullableStringPtr(s *string) *string {
	if s == nil || *s == "" {
		return nil
	}
	return s
}

func nullableAlpn(s string) *string {
	switch s {
	case "h3", "h2", "http/1.1", "h2,http/1.1", "h3,h2,http/1.1", "h3,h2":
		return &s
	default:
		return nil
	}
}

func nullableMihomoIpVersion(s string) *string {
	switch s {
	case "dual", "ipv4", "ipv6", "ipv4-prefer", "ipv6-prefer":
		return &s
	default:
		return nil
	}
}

func formatHostResponse(db *gorm.DB, h *database.Host) (map[string]interface{}, error) {
	viewPos := h.ViewPosition
	if viewPos <= 0 {
		viewPos = 1
	}

	secLayer := h.SecurityLayer
	switch secLayer {
	case "DEFAULT", "TLS", "NONE":
	default:
		secLayer = "DEFAULT"
	}

	var xhttpExtra, muxParams, sockoptParams, finalMask interface{}
	if h.XhttpExtraParams != "" && h.XhttpExtraParams != "null" {
		_ = json.Unmarshal([]byte(h.XhttpExtraParams), &xhttpExtra)
	}
	if h.MuxParams != "" && h.MuxParams != "null" {
		_ = json.Unmarshal([]byte(h.MuxParams), &muxParams)
	}
	if h.SockoptParams != "" && h.SockoptParams != "null" {
		_ = json.Unmarshal([]byte(h.SockoptParams), &sockoptParams)
	}
	if h.FinalMask != "" && h.FinalMask != "null" {
		_ = json.Unmarshal([]byte(h.FinalMask), &finalMask)
	}

	tags := []string(h.Tags)
	if tags == nil {
		tags = []string{}
	}

	var nodes []string
	if err := db.Model(&database.HostsToNode{}).
		Where("host_uuid = ?", h.UUID).
		Order("node_uuid ASC").Pluck("node_uuid", &nodes).Error; err != nil {
		return nil, err
	}
	if nodes == nil {
		nodes = []string{}
	}

	excludeTypes := []string(h.ExcludeFromSubscriptionTypes)
	if excludeTypes == nil {
		excludeTypes = []string{}
	}

	mapper := map[string]interface{}{}
	if h.Mapper != "" && h.Mapper != "null" {
		_ = json.Unmarshal([]byte(h.Mapper), &mapper)
	}
	if mapper == nil {
		mapper = map[string]interface{}{}
	}

	var squads []string
	if err := db.Model(&database.InternalSquadHostLink{}).
		Where("host_uuid = ?", h.UUID).
		Order("squad_uuid ASC").Pluck("squad_uuid", &squads).Error; err != nil {
		return nil, err
	}
	if squads == nil {
		squads = []string{}
	}

	squadsMode := h.InternalSquadsMode
	if squadsMode != "ALLOW_ONLY" {
		squadsMode = "EXCLUDE"
	}

	fp := nullableString(h.Fingerprint)
	if fp == nil {
		defaultFp := "chrome"
		fp = &defaultFp
	}

	return map[string]interface{}{
		"uuid":             h.UUID,
		"viewPosition":     viewPos,
		"remark":           h.Remark,
		"address":          h.Address,
		"port":             h.Port,
		"path":             nullableString(h.Path),
		"sni":              nullableString(h.Sni),
		"host":             nullableString(h.Host),
		"alpn":             nullableAlpn(h.Alpn),
		"fingerprint":      fp,
		"isDisabled":       h.IsDisabled,
		"securityLayer":    secLayer,
		"xhttpExtraParams": xhttpExtra,
		"muxParams":        muxParams,
		"sockoptParams":    sockoptParams,
		"finalMask":        finalMask,
		"inbound": map[string]interface{}{
			"configProfileUuid":        nullableStringPtr(h.ConfigProfileUUID),
			"configProfileInboundUuid": nullableStringPtr(h.ConfigProfileInboundUUID),
		},
		"serverDescription":            nullableString(h.ServerDescription),
		"tags":                         tags,
		"isHidden":                     h.IsHidden,
		"overrideSniFromAddress":       h.OverrideSniFromAddress,
		"keepSniBlank":                 h.KeepSniBlank,
		"vlessRouteId":                 h.VlessRouteId,
		"pinnedPeerCertSha256":         nullableString(h.PinnedPeerCertSha256),
		"verifyPeerCertByName":         nullableString(h.VerifyPeerCertByName),
		"shuffleHost":                  h.ShuffleHost,
		"mihomoX25519":                 h.MihomoX25519,
		"mihomoIpVersion":              nullableMihomoIpVersion(h.MihomoIpVersion),
		"nodes":                        nodes,
		"xrayJsonTemplateUuid":         nullableStringPtr(h.XrayJsonTemplateUUID),
		"excludeFromSubscriptionTypes": excludeTypes,
		"mapper":                       mapper,
		"internalSquads": map[string]interface{}{
			"mode":   squadsMode,
			"squads": squads,
		},
	}, nil
}

func (h *Handler) GetHosts(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	hostsList, err := h.service.GetAllHosts()
	if err != nil {
		http.Error(w, `{"message":"Failed to fetch hosts"}`, http.StatusInternalServerError)
		return
	}

	res := make([]map[string]interface{}, 0, len(hostsList))
	for i := range hostsList {
		formatted, err := formatHostResponse(h.service.DB(), &hostsList[i])
		if err != nil {
			http.Error(w, `{"message":"Failed to load host relations"}`, http.StatusInternalServerError)
			return
		}
		res = append(res, formatted)
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": res,
	})
}

func (h *Handler) CreateHost(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var input CreateHostInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, `{"message":"Invalid request body"}`, http.StatusBadRequest)
		return
	}

	host, err := h.service.CreateHostFromInput(input)
	if err != nil {
		http.Error(w, `{"message":"Failed to create host"}`, http.StatusInternalServerError)
		return
	}

	formatted, err := formatHostResponse(h.service.DB(), host)
	if err != nil {
		http.Error(w, `{"message":"Failed to load host relations"}`, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": formatted,
	})
}

func (h *Handler) GetHost(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	uuidParam := chi.URLParam(r, "uuid")
	host, err := h.service.GetHostByUUID(uuidParam)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Host not found"})
		return
	}
	formatted, err := formatHostResponse(h.service.DB(), host)
	if err != nil {
		http.Error(w, `{"message":"Failed to load host relations"}`, http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"response": formatted})
}

func (h *Handler) UpdateHost(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var input struct {
		UUID    string `json:"uuid"`
		Inbound *struct {
			ConfigProfileUUID        string `json:"configProfileUuid"`
			ConfigProfileInboundUUID string `json:"configProfileInboundUuid"`
		} `json:"inbound"`
		ConfigProfileUUID            *string         `json:"configProfileUuid"`
		ConfigProfileInboundUUID     *string         `json:"configProfileInboundUuid"`
		Remark                       *string         `json:"remark"`
		Address                      *string         `json:"address"`
		Port                         *int            `json:"port"`
		Path                         *string         `json:"path"`
		Sni                          *string         `json:"sni"`
		Host                         *string         `json:"host"`
		Alpn                         *string         `json:"alpn"`
		Fingerprint                  *string         `json:"fingerprint"`
		IsDisabled                   *bool           `json:"isDisabled"`
		SecurityLayer                *string         `json:"securityLayer"`
		XhttpExtraParams             json.RawMessage `json:"xhttpExtraParams"`
		MuxParams                    json.RawMessage `json:"muxParams"`
		SockoptParams                json.RawMessage `json:"sockoptParams"`
		FinalMask                    json.RawMessage `json:"finalMask"`
		ServerDescription            *string         `json:"serverDescription"`
		Tags                         *[]string       `json:"tags"`
		IsHidden                     *bool           `json:"isHidden"`
		OverrideSniFromAddress       *bool           `json:"overrideSniFromAddress"`
		KeepSniBlank                 *bool           `json:"keepSniBlank"`
		PinnedPeerCertSha256         *string         `json:"pinnedPeerCertSha256"`
		VerifyPeerCertByName         *string         `json:"verifyPeerCertByName"`
		VlessRouteId                 *int            `json:"vlessRouteId"`
		ShuffleHost                  *bool           `json:"shuffleHost"`
		MihomoX25519                 *bool           `json:"mihomoX25519"`
		MihomoIpVersion              *string         `json:"mihomoIpVersion"`
		Nodes                        *[]string       `json:"nodes"`
		XrayJsonTemplateUUID         *string         `json:"xrayJsonTemplateUuid"`
		ExcludeFromSubscriptionTypes *[]string       `json:"excludeFromSubscriptionTypes"`
		Mapper                       json.RawMessage `json:"mapper"`
		InternalSquads               *struct {
			Mode   string   `json:"mode"`
			Squads []string `json:"squads"`
		} `json:"internalSquads"`
	}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, `{"message":"Invalid request body"}`, http.StatusBadRequest)
		return
	}
	targetUUID := input.UUID
	if targetUUID == "" {
		targetUUID = chi.URLParam(r, "uuid")
	}

	host, err := h.service.GetHostByUUID(targetUUID)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Host not found"})
		return
	}

	if input.Inbound != nil {
		if input.Inbound.ConfigProfileUUID != "" {
			host.ConfigProfileUUID = nullableString(input.Inbound.ConfigProfileUUID)
		}
		if input.Inbound.ConfigProfileInboundUUID != "" {
			host.ConfigProfileInboundUUID = nullableString(input.Inbound.ConfigProfileInboundUUID)
		}
	}
	if input.ConfigProfileUUID != nil {
		host.ConfigProfileUUID = nullableStringPtr(input.ConfigProfileUUID)
	}
	if input.ConfigProfileInboundUUID != nil {
		host.ConfigProfileInboundUUID = nullableStringPtr(input.ConfigProfileInboundUUID)
	}
	if input.Remark != nil {
		host.Remark = *input.Remark
	}
	if input.Address != nil {
		host.Address = strings.TrimSpace(*input.Address)
	}
	if input.Port != nil {
		host.Port = *input.Port
	}
	if input.Path != nil {
		host.Path = *input.Path
	}
	if input.Sni != nil {
		host.Sni = *input.Sni
	}
	if input.Host != nil {
		host.Host = *input.Host
	}
	if input.Alpn != nil {
		host.Alpn = *input.Alpn
	}
	if input.Fingerprint != nil {
		host.Fingerprint = *input.Fingerprint
	}
	if input.IsDisabled != nil {
		host.IsDisabled = *input.IsDisabled
	}
	if input.SecurityLayer != nil {
		host.SecurityLayer = *input.SecurityLayer
	}
	if len(input.XhttpExtraParams) > 0 {
		host.XhttpExtraParams = string(input.XhttpExtraParams)
	}
	if len(input.MuxParams) > 0 {
		host.MuxParams = string(input.MuxParams)
	}
	if len(input.SockoptParams) > 0 {
		host.SockoptParams = string(input.SockoptParams)
	}
	if len(input.FinalMask) > 0 {
		host.FinalMask = string(input.FinalMask)
	}
	if input.ServerDescription != nil {
		host.ServerDescription = *input.ServerDescription
	}
	if input.Tags != nil {
		host.Tags = database.StringArray(*input.Tags)
	}
	if input.IsHidden != nil {
		host.IsHidden = *input.IsHidden
	}
	if input.OverrideSniFromAddress != nil {
		host.OverrideSniFromAddress = *input.OverrideSniFromAddress
	}
	if input.KeepSniBlank != nil {
		host.KeepSniBlank = *input.KeepSniBlank
	}
	if input.PinnedPeerCertSha256 != nil {
		host.PinnedPeerCertSha256 = *input.PinnedPeerCertSha256
	}
	if input.VerifyPeerCertByName != nil {
		host.VerifyPeerCertByName = *input.VerifyPeerCertByName
	}
	if input.VlessRouteId != nil {
		host.VlessRouteId = input.VlessRouteId
	}
	if input.ShuffleHost != nil {
		host.ShuffleHost = *input.ShuffleHost
	}
	if input.MihomoX25519 != nil {
		host.MihomoX25519 = *input.MihomoX25519
	}
	if input.MihomoIpVersion != nil {
		host.MihomoIpVersion = *input.MihomoIpVersion
	}
	if input.Nodes != nil {
		nb, _ := json.Marshal(*input.Nodes)
		host.Nodes = string(nb)
	}
	if input.XrayJsonTemplateUUID != nil {
		host.XrayJsonTemplateUUID = nullableStringPtr(input.XrayJsonTemplateUUID)
	}
	if input.ExcludeFromSubscriptionTypes != nil {
		host.ExcludeFromSubscriptionTypes = database.StringArray(*input.ExcludeFromSubscriptionTypes)
	}
	if len(input.Mapper) > 0 {
		host.Mapper = string(input.Mapper)
	}
	if input.InternalSquads != nil {
		if input.InternalSquads.Mode != "" {
			host.InternalSquadsMode = input.InternalSquads.Mode
		}
		sb, _ := json.Marshal(input.InternalSquads.Squads)
		host.InternalSquads = string(sb)
	}
	if err := h.service.DB().Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(host).Error; err != nil {
			return err
		}
		if input.Nodes == nil && input.InternalSquads == nil {
			return nil
		}

		nodeUUIDs := []string{}
		if input.Nodes != nil {
			nodeUUIDs = *input.Nodes
		} else if err := tx.Model(&database.HostsToNode{}).Where("host_uuid = ?", host.UUID).Pluck("node_uuid", &nodeUUIDs).Error; err != nil {
			return err
		}

		squadUUIDs := []string{}
		if input.InternalSquads != nil {
			squadUUIDs = input.InternalSquads.Squads
		} else if err := tx.Model(&database.InternalSquadHostLink{}).Where("host_uuid = ?", host.UUID).Pluck("squad_uuid", &squadUUIDs).Error; err != nil {
			return err
		}
		return replaceHostRelations(tx, host.UUID, nodeUUIDs, squadUUIDs)
	}); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Failed to update host"})
		return
	}

	formatted, err := formatHostResponse(h.service.DB(), host)
	if err != nil {
		http.Error(w, `{"message":"Failed to load host relations"}`, http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": formatted,
	})
}

func (h *Handler) DeleteHost(w http.ResponseWriter, r *http.Request) {
	uuid := chi.URLParam(r, "uuid")
	if err := h.service.DeleteHost(uuid); err != nil {
		http.Error(w, `{"message":"Failed to delete host"}`, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) CloneHost(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var body struct {
		CloneFromUuid string `json:"cloneFromUuid"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.CloneFromUuid == "" {
		http.Error(w, `{"message":"Invalid body"}`, http.StatusBadRequest)
		return
	}

	var host database.Host
	if err := h.service.DB().Where("uuid = ?", body.CloneFromUuid).First(&host).Error; err != nil {
		http.Error(w, `{"message":"Host not found"}`, http.StatusNotFound)
		return
	}

	host.UUID = uuid.NewString()
	host.Remark = host.Remark + " (Clone)"

	if err := h.service.DB().Create(&host).Error; err != nil {
		http.Error(w, `{"message":"Failed to clone"}`, http.StatusInternalServerError)
		return
	}

	formatted, _ := formatHostResponse(h.service.DB(), &host)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": formatted,
	})
}

func (h *Handler) ReorderHosts(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var body struct {
		Hosts []struct {
			UUID         string `json:"uuid"`
			ViewPosition int    `json:"viewPosition"`
		} `json:"hosts"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)

	for _, item := range body.Hosts {
		if item.UUID != "" {
			h.service.DB().Model(&database.Host{}).Where("uuid = ?", item.UUID).Update("view_position", item.ViewPosition)
		}
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": map[string]interface{}{
			"isUpdated": true,
		},
	})
}

func (h *Handler) BulkHostsAction(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	path := r.URL.Path

	if strings.HasSuffix(path, "/delete") || strings.HasSuffix(path, "/enable") || strings.HasSuffix(path, "/disable") {
		var body struct {
			UUIDs []string `json:"uuids"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, `{"message":"Invalid request body"}`, http.StatusBadRequest)
			return
		}
		if len(body.UUIDs) > 0 {
			if strings.HasSuffix(path, "/delete") {
				for _, uid := range body.UUIDs {
					_ = h.service.DeleteHost(uid)
				}
			} else if strings.HasSuffix(path, "/disable") {
				h.service.DB().Model(&database.Host{}).Where("uuid IN ?", body.UUIDs).Update("is_disabled", true)
			} else if strings.HasSuffix(path, "/enable") {
				h.service.DB().Model(&database.Host{}).Where("uuid IN ?", body.UUIDs).Update("is_disabled", false)
			}
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if strings.HasSuffix(path, "/update") {
		var body struct {
			UUIDs                        []string `json:"uuids"`
			Port                         *int     `json:"port"`
			Tags                         []string `json:"tags"`
			InternalSquadsMode           *string  `json:"internalSquadsMode"`
			ExcludeFromSubscriptionTypes []string `json:"excludeFromSubscriptionTypes"`
		}

		if err := json.NewDecoder(r.Body).Decode(&body); err == nil && len(body.UUIDs) > 0 {
			updates := map[string]interface{}{}
			if body.Port != nil {
				updates["port"] = *body.Port
			}
			if body.InternalSquadsMode != nil {
				updates["internal_squads_mode"] = *body.InternalSquadsMode
			}
			if body.Tags != nil {
				updates["tags"] = database.StringArray(body.Tags)
			}
			if body.ExcludeFromSubscriptionTypes != nil {
				updates["exclude_from_subscription_types"] = database.StringArray(body.ExcludeFromSubscriptionTypes)
			}
			if len(updates) > 0 {
				h.service.DB().Model(&database.Host{}).Where("uuid IN ?", body.UUIDs).Updates(updates)
			}
		}
		w.WriteHeader(http.StatusNoContent)
		return
	}
}

func (h *Handler) Preview(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var req PreviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"message":"Invalid request body"}`, http.StatusBadRequest)
		return
	}

	if req.UserAgent == "" {
		req.UserAgent = r.Header.Get("User-Agent")
	}

	preview, err := h.service.GenerateHostPreview(req)
	if err != nil {
		http.Error(w, `{"message":"`+err.Error()+`"}`, http.StatusBadRequest)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": preview,
	})
}
