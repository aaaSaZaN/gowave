package configprofiles

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"remnawave-go/internal/database"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type NodeProfileSyncer interface {
	RestartNodesByProfile(profileUUID string)
}

type Service struct {
	db     *gorm.DB
	syncer NodeProfileSyncer
}

func NewService(db *gorm.DB, syncer ...NodeProfileSyncer) *Service {
	var s NodeProfileSyncer
	if len(syncer) > 0 {
		s = syncer[0]
	}
	return &Service{db: db, syncer: s}
}

func (s *Service) SetSyncer(syncer NodeProfileSyncer) {
	s.syncer = syncer
}

func (s *Service) DB() *gorm.DB {
	return s.db
}

type CreateHostDTO struct {
	Remark                   string `json:"remark"`
	Address                  string `json:"address"`
	Port                     int    `json:"port"`
	Path                     string `json:"path"`
	Sni                      string `json:"sni"`
	Host                     string `json:"host"`
	Alpn                     string `json:"alpn"`
	Fingerprint              string `json:"fingerprint"`
	SecurityLayer            string `json:"securityLayer"`
	ConfigProfileUUID        string `json:"configProfileUuid"`
	ConfigProfileInboundUUID string `json:"configProfileInboundUuid"`
}

type CreateHostInput struct {
	Inbound struct {
		ConfigProfileUUID        string `json:"configProfileUuid"`
		ConfigProfileInboundUUID string `json:"configProfileInboundUuid"`
	} `json:"inbound"`
	ConfigProfileUUID            *string         `json:"configProfileUuid"`
	ConfigProfileInboundUUID     *string         `json:"configProfileInboundUuid"`
	Remark                       string          `json:"remark"`
	Address                      string          `json:"address"`
	Port                         int             `json:"port"`
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
	Tags                         []string        `json:"tags"`
	IsHidden                     *bool           `json:"isHidden"`
	OverrideSniFromAddress       *bool           `json:"overrideSniFromAddress"`
	KeepSniBlank                 *bool           `json:"keepSniBlank"`
	PinnedPeerCertSha256         *string         `json:"pinnedPeerCertSha256"`
	VerifyPeerCertByName         *string         `json:"verifyPeerCertByName"`
	VlessRouteId                 *int            `json:"vlessRouteId"`
	ShuffleHost                  *bool           `json:"shuffleHost"`
	MihomoX25519                 *bool           `json:"mihomoX25519"`
	MihomoIpVersion              *string         `json:"mihomoIpVersion"`
	Nodes                        []string        `json:"nodes"`
	XrayJsonTemplateUUID         *string         `json:"xrayJsonTemplateUuid"`
	ExcludeFromSubscriptionTypes []string        `json:"excludeFromSubscriptionTypes"`
	Mapper                       json.RawMessage `json:"mapper"`
	InternalSquads               *struct {
		Mode   string   `json:"mode"`
		Squads []string `json:"squads"`
	} `json:"internalSquads"`
}

func (s *Service) CreateHostFromInput(input CreateHostInput) (*database.Host, error) {
	var count int64
	s.db.Model(&database.Host{}).Count(&count)

	profileUUID := input.Inbound.ConfigProfileUUID
	if profileUUID == "" && input.ConfigProfileUUID != nil {
		profileUUID = *input.ConfigProfileUUID
	}
	inboundUUID := input.Inbound.ConfigProfileInboundUUID
	if inboundUUID == "" && input.ConfigProfileInboundUUID != nil {
		inboundUUID = *input.ConfigProfileInboundUUID
	}

	port := input.Port
	if port == 0 {
		port = 443
	}

	// Upstream stores a missing fingerprint as null; links then fall back to
	// the inbound's own fingerprint.
	fp := ""
	if input.Fingerprint != nil {
		fp = *input.Fingerprint
	}

	secLayer := "DEFAULT"
	if input.SecurityLayer != nil && *input.SecurityLayer != "" {
		secLayer = *input.SecurityLayer
	}

	var path, sni, hostStr, alpnStr, serverDesc, pinnedCert, verifyName, mihomoIp, xrayTmpl string
	if input.Path != nil {
		path = *input.Path
	}
	if input.Sni != nil {
		sni = *input.Sni
	}
	if input.Host != nil {
		hostStr = *input.Host
	}
	if input.Alpn != nil {
		alpnStr = *input.Alpn
	}
	if input.ServerDescription != nil {
		serverDesc = *input.ServerDescription
	}
	if input.PinnedPeerCertSha256 != nil {
		pinnedCert = *input.PinnedPeerCertSha256
	}
	if input.VerifyPeerCertByName != nil {
		verifyName = *input.VerifyPeerCertByName
	}
	if input.MihomoIpVersion != nil {
		mihomoIp = *input.MihomoIpVersion
	}
	if input.XrayJsonTemplateUUID != nil {
		xrayTmpl = *input.XrayJsonTemplateUUID
	}
	var xrayTmplUUID *string
	if xrayTmpl != "" {
		xrayTmplUUID = &xrayTmpl
	}
	var profileUUIDValue, inboundUUIDValue *string
	if profileUUID != "" {
		profileUUIDValue = &profileUUID
	}
	if inboundUUID != "" {
		inboundUUIDValue = &inboundUUID
	}

	var isDisabled, isHidden, overrideSni, keepSni, shuffleHost, mihomoX25519 bool
	if input.IsDisabled != nil {
		isDisabled = *input.IsDisabled
	}
	if input.IsHidden != nil {
		isHidden = *input.IsHidden
	}
	if input.OverrideSniFromAddress != nil {
		overrideSni = *input.OverrideSniFromAddress
	}
	if input.KeepSniBlank != nil {
		keepSni = *input.KeepSniBlank
	}
	if input.ShuffleHost != nil {
		shuffleHost = *input.ShuffleHost
	}
	if input.MihomoX25519 != nil {
		mihomoX25519 = *input.MihomoX25519
	}

	nodesBytes, _ := json.Marshal(input.Nodes)

	mapperStr := "{}"
	if len(input.Mapper) > 0 && string(input.Mapper) != "null" {
		mapperStr = string(input.Mapper)
	}

	squadsMode := "EXCLUDE"
	squadsStr := "[]"
	if input.InternalSquads != nil {
		if input.InternalSquads.Mode != "" {
			squadsMode = input.InternalSquads.Mode
		}
		sb, _ := json.Marshal(input.InternalSquads.Squads)
		squadsStr = string(sb)
	}

	now := time.Now().UTC()
	h := &database.Host{
		UUID:                         uuid.NewString(),
		ViewPosition:                 int(count) + 1,
		Remark:                       input.Remark,
		Address:                      strings.TrimSpace(input.Address),
		Port:                         port,
		Path:                         path,
		Sni:                          sni,
		Host:                         hostStr,
		Alpn:                         alpnStr,
		Fingerprint:                  fp,
		SecurityLayer:                secLayer,
		XhttpExtraParams:             string(input.XhttpExtraParams),
		MuxParams:                    string(input.MuxParams),
		SockoptParams:                string(input.SockoptParams),
		FinalMask:                    string(input.FinalMask),
		IsDisabled:                   isDisabled,
		ServerDescription:            serverDesc,
		VlessRouteId:                 input.VlessRouteId,
		PinnedPeerCertSha256:         pinnedCert,
		VerifyPeerCertByName:         verifyName,
		ShuffleHost:                  shuffleHost,
		MihomoX25519:                 mihomoX25519,
		MihomoIpVersion:              mihomoIp,
		XrayJsonTemplateUUID:         xrayTmplUUID,
		KeepSniBlank:                 keepSni,
		ExcludeFromSubscriptionTypes: database.StringArray(input.ExcludeFromSubscriptionTypes),
		Mapper:                       mapperStr,
		InternalSquadsMode:           squadsMode,
		InternalSquads:               squadsStr,
		Tags:                         database.StringArray(input.Tags),
		Nodes:                        string(nodesBytes),
		IsHidden:                     isHidden,
		OverrideSniFromAddress:       overrideSni,
		ConfigProfileUUID:            profileUUIDValue,
		ConfigProfileInboundUUID:     inboundUUIDValue,
		CreatedAt:                    now,
		UpdatedAt:                    now,
	}

	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(h).Error; err != nil {
			return err
		}
		var squadUUIDs []string
		if input.InternalSquads != nil {
			squadUUIDs = input.InternalSquads.Squads
		}
		return replaceHostRelations(tx, h.UUID, input.Nodes, squadUUIDs)
	})
	if err != nil {
		return nil, err
	}
	return h, nil
}

func replaceHostRelations(tx *gorm.DB, hostUUID string, nodeUUIDs, squadUUIDs []string) error {
	if err := tx.Where("host_uuid = ?", hostUUID).Delete(&database.HostsToNode{}).Error; err != nil {
		return err
	}
	if err := tx.Where("host_uuid = ?", hostUUID).Delete(&database.InternalSquadHostLink{}).Error; err != nil {
		return err
	}

	nodeLinks := make([]database.HostsToNode, 0, len(nodeUUIDs))
	seenNodes := make(map[string]struct{}, len(nodeUUIDs))
	for _, uuid := range nodeUUIDs {
		if uuid == "" {
			continue
		}
		if _, exists := seenNodes[uuid]; exists {
			continue
		}
		seenNodes[uuid] = struct{}{}
		nodeLinks = append(nodeLinks, database.HostsToNode{HostUUID: hostUUID, NodeUUID: uuid})
	}
	if len(nodeLinks) > 0 {
		if err := tx.Create(&nodeLinks).Error; err != nil {
			return err
		}
	}

	squadLinks := make([]database.InternalSquadHostLink, 0, len(squadUUIDs))
	seenSquads := make(map[string]struct{}, len(squadUUIDs))
	for _, uuid := range squadUUIDs {
		if uuid == "" {
			continue
		}
		if _, exists := seenSquads[uuid]; exists {
			continue
		}
		seenSquads[uuid] = struct{}{}
		squadLinks = append(squadLinks, database.InternalSquadHostLink{HostUUID: hostUUID, SquadUUID: uuid})
	}
	if len(squadLinks) > 0 {
		if err := tx.Create(&squadLinks).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) CreateHost(dto CreateHostDTO) (*database.Host, error) {
	var input CreateHostInput
	input.Remark = dto.Remark
	input.Address = dto.Address
	input.Port = dto.Port
	input.Path = &dto.Path
	input.Sni = &dto.Sni
	input.Host = &dto.Host
	input.Alpn = &dto.Alpn
	input.Fingerprint = &dto.Fingerprint
	input.SecurityLayer = &dto.SecurityLayer
	input.ConfigProfileUUID = &dto.ConfigProfileUUID
	input.ConfigProfileInboundUUID = &dto.ConfigProfileInboundUUID
	return s.CreateHostFromInput(input)
}

func (s *Service) GetAllHosts() ([]database.Host, error) {
	var hosts []database.Host
	err := s.db.Order("view_position asc").Find(&hosts).Error
	return hosts, err
}

func (s *Service) GetHostByUUID(uuid string) (*database.Host, error) {
	var host database.Host
	err := s.db.Where("uuid = ?", uuid).First(&host).Error
	if err != nil {
		return nil, err
	}
	return &host, nil
}

func (s *Service) UpdateHost(uuid string, updates map[string]interface{}) (*database.Host, error) {
	updates["updated_at"] = time.Now().UTC()
	err := s.db.Model(&database.Host{}).Where("uuid = ?", uuid).Updates(updates).Error
	if err != nil {
		return nil, err
	}
	return s.GetHostByUUID(uuid)
}

func (s *Service) DeleteHost(uuid string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("host_uuid = ?", uuid).Delete(&database.HostsToNode{}).Error; err != nil {
			return err
		}
		if err := tx.Where("host_uuid = ?", uuid).Delete(&database.InternalSquadHostLink{}).Error; err != nil {
			return err
		}
		return tx.Where("uuid = ?", uuid).Delete(&database.Host{}).Error
	})
}

func (s *Service) GetAllProfiles() ([]database.ConfigProfile, error) {
	var profiles []database.ConfigProfile
	err := s.db.Order("view_position asc").Find(&profiles).Error
	return profiles, err
}

func (s *Service) GetProfileByUUID(uuid string) (*database.ConfigProfile, error) {
	var profile database.ConfigProfile
	err := s.db.Where("uuid = ?", uuid).First(&profile).Error
	if err != nil {
		return nil, err
	}
	return &profile, nil
}

func (s *Service) CreateProfile(name string, configRaw []byte) (*database.ConfigProfile, error) {
	var count int64
	s.db.Model(&database.ConfigProfile{}).Where("name = ?", name).Count(&count)
	if count > 0 {
		return nil, fmt.Errorf("config profile with name '%s' already exists", name)
	}

	sortedBytes, err := SortXrayConfig(configRaw)
	if err != nil {
		return nil, fmt.Errorf("invalid config json: %w", err)
	}

	var maxPos int
	s.db.Model(&database.ConfigProfile{}).Select("COALESCE(MAX(view_position), 0)").Scan(&maxPos)

	profileUUID := uuid.NewString()
	now := time.Now().UTC()

	profile := &database.ConfigProfile{
		UUID:         profileUUID,
		ViewPosition: maxPos + 1,
		Name:         name,
		Tags:         database.StringArray{},
		Config:       string(sortedBytes),
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(profile).Error; err != nil {
			return err
		}
		return syncInbounds(tx, profileUUID, sortedBytes, true)
	}); err != nil {
		return nil, err
	}
	return profile, nil
}

func (s *Service) UpdateProfile(uuidParam string, name *string, configRaw []byte) (*database.ConfigProfile, error) {
	profile, err := s.GetProfileByUUID(uuidParam)
	if err != nil {
		return nil, err
	}

	updates := map[string]interface{}{
		"updated_at": time.Now().UTC(),
	}

	if name != nil && *name != "" {
		updates["name"] = *name
	}

	var sortedBytes []byte
	if len(configRaw) > 0 {
		var err error
		sortedBytes, err = SortXrayConfig(configRaw)
		if err != nil {
			return nil, fmt.Errorf("invalid config json: %w", err)
		}
		updates["config"] = string(sortedBytes)
	}

	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&database.ConfigProfile{}).Where("uuid = ?", profile.UUID).Updates(updates).Error; err != nil {
			return err
		}
		if len(sortedBytes) > 0 {
			return syncInbounds(tx, profile.UUID, sortedBytes, false)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	if len(sortedBytes) > 0 {
		if s.syncer != nil {
			go s.syncer.RestartNodesByProfile(profile.UUID)
		} else {
			log.Printf("[CONFIG PROFILE] Warning: syncer is nil, skipping RestartNodesByProfile for %s", profile.UUID)
		}
	}

	return s.GetProfileByUUID(profile.UUID)
}

func (s *Service) DeleteProfile(uuidParam string) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("profile_uuid = ?", uuidParam).Delete(&database.ConfigProfileInbound{}).Error; err != nil {
			return err
		}
		return tx.Where("uuid = ?", uuidParam).Delete(&database.ConfigProfile{}).Error
	})
}

func syncInbounds(tx *gorm.DB, profileUUID string, configBytes []byte, creating bool) error {
	var root struct {
		Inbounds []json.RawMessage `json:"inbounds"`
	}
	if err := json.Unmarshal(configBytes, &root); err != nil {
		if creating {
			return nil
		}
		return err
	}
	if root.Inbounds == nil {
		if creating {
			return nil
		}
	}

	var existing []database.ConfigProfileInbound
	if err := tx.Where("profile_uuid = ?", profileUUID).Find(&existing).Error; err != nil {
		return err
	}
	existingByTag := make(map[string]database.ConfigProfileInbound, len(existing))
	for _, inbound := range existing {
		existingByTag[inbound.Tag] = inbound
	}

	seenTags := make(map[string]struct{}, len(root.Inbounds))
	for _, itemRaw := range root.Inbounds {
		var meta struct {
			Tag            string      `json:"tag"`
			Protocol       string      `json:"protocol"`
			Port           interface{} `json:"port"`
			StreamSettings struct {
				Network  *string `json:"network"`
				Security *string `json:"security"`
			} `json:"streamSettings"`
		}
		if err := json.Unmarshal(itemRaw, &meta); err != nil {
			continue
		}

		tag := meta.Tag
		proto := meta.Protocol
		if strings.TrimSpace(tag) == "" || strings.TrimSpace(proto) == "" {
			return fmt.Errorf("inbound tag and protocol are required")
		}
		if _, exists := seenTags[tag]; exists {
			return fmt.Errorf("duplicate inbound tag %q", tag)
		}
		seenTags[tag] = struct{}{}

		var network *string = meta.StreamSettings.Network
		var security *string = meta.StreamSettings.Security
		if network != nil && *network == "" {
			network = nil
		}
		if security != nil && *security == "" {
			security = nil
		}

		var port *int
		switch p := meta.Port.(type) {
		case float64:
			pInt := int(p)
			port = &pInt
		case int:
			port = &p
		case string:
			if first := strings.Split(p, ",")[0]; first != "" {
				var pInt int
				if _, err := fmt.Sscanf(first, "%d", &pInt); err == nil {
					port = &pInt
				}
			}
		}

		sortedRaw, err := SortInboundConfig(itemRaw)
		if err != nil {
			sortedRaw = itemRaw
		}

		ibUUID := uuid.NewString()
		inbound := database.ConfigProfileInbound{
			UUID:        ibUUID,
			ProfileUUID: profileUUID,
			Tag:         tag,
			Type:        proto,
			Network:     network,
			Security:    security,
			Port:        port,
			RawInbound:  string(sortedRaw),
		}
		if previous, exists := existingByTag[tag]; exists {
			inbound.UUID = previous.UUID
			if err := tx.Model(&database.ConfigProfileInbound{}).Where("uuid = ?", previous.UUID).Updates(map[string]interface{}{
				"tag":         inbound.Tag,
				"type":        inbound.Type,
				"network":     inbound.Network,
				"security":    inbound.Security,
				"port":        inbound.Port,
				"raw_inbound": inbound.RawInbound,
			}).Error; err != nil {
				return err
			}
		} else if err := tx.Create(&inbound).Error; err != nil {
			return err
		}
	}
	for _, oldInbound := range existing {
		if _, stillExists := seenTags[oldInbound.Tag]; stillExists {
			continue
		}
		if err := tx.Where("uuid = ?", oldInbound.UUID).Delete(&database.ConfigProfileInbound{}).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) GetInboundsByProfileUUID(uuidParam string) ([]database.ConfigProfileInbound, error) {
	var inbounds []database.ConfigProfileInbound
	err := s.db.Where("profile_uuid = ?", uuidParam).Find(&inbounds).Error
	return inbounds, err
}

func (s *Service) GetAllInbounds() ([]database.ConfigProfileInbound, error) {
	var inbounds []database.ConfigProfileInbound
	err := s.db.Find(&inbounds).Error
	return inbounds, err
}

func (s *Service) SetProfileTags(uuidParam string, tags []string) error {
	return s.db.Model(&database.ConfigProfile{}).Where("uuid = ?", uuidParam).Update("tags", database.StringArray(tags)).Error
}

type PreviewRequest struct {
	HostUUID  string         `json:"hostUuid,omitempty"`
	Host      *CreateHostDTO `json:"host,omitempty"`
	UserAgent string         `json:"userAgent"`
	Protocol  string         `json:"protocol"`
}

type PreviewResponse struct {
	ClientType     string            `json:"clientType"`
	ContentType    string            `json:"contentType"`
	Headers        map[string]string `json:"headers"`
	PreviewContent string            `json:"previewContent"`
}

func (s *Service) GenerateHostPreview(req PreviewRequest) (*PreviewResponse, error) {
	var h CreateHostDTO
	if req.Host != nil {
		h = *req.Host
	} else if req.HostUUID != "" {
		existing, err := s.GetHostByUUID(req.HostUUID)
		if err != nil {
			return nil, err
		}
		h = CreateHostDTO{
			Remark:        existing.Remark,
			Address:       existing.Address,
			Port:          existing.Port,
			Path:          existing.Path,
			Sni:           existing.Sni,
			Host:          existing.Host,
			Alpn:          existing.Alpn,
			Fingerprint:   existing.Fingerprint,
			SecurityLayer: existing.SecurityLayer,
		}
	} else {
		return nil, fmt.Errorf("host details or hostUuid must be provided")
	}

	if h.Port == 0 {
		h.Port = 443
	}
	if h.Fingerprint == "" {
		h.Fingerprint = "chrome"
	}

	proto := strings.ToLower(req.Protocol)
	if proto == "" {
		proto = "vless"
	}

	ua := strings.ToLower(req.UserAgent)
	clientType := "Base64 / Standard"
	contentType := "text/plain; charset=utf-8"

	sampleUUID := "11111111-2222-3333-4444-555555555555"

	var content string

	if strings.Contains(ua, "clash") {
		clientType = "Clash / Mihomo"
		contentType = "text/yaml; charset=utf-8"
		content = fmt.Sprintf(`proxies:
  - name: "%s"
    type: %s
    server: %s
    port: %d
    uuid: %s
    network: ws
    tls: true
    servername: %s
    client-fingerprint: %s
`, h.Remark, proto, h.Address, h.Port, sampleUUID, h.Sni, h.Fingerprint)
	} else if strings.Contains(ua, "sing-box") || strings.Contains(ua, "singbox") {
		clientType = "Sing-Box"
		contentType = "application/json; charset=utf-8"
		content = fmt.Sprintf(`{
  "outbounds": [
    {
      "type": "%s",
      "tag": "%s",
      "server": "%s",
      "server_port": %d,
      "uuid": "%s",
      "tls": {
        "enabled": true,
        "server_name": "%s",
        "utls": {
          "enabled": true,
          "fingerprint": "%s"
        }
      }
    }
  ]
}`, proto, h.Remark, h.Address, h.Port, sampleUUID, h.Sni, h.Fingerprint)
	} else {
		security := "tls"
		if h.SecurityLayer != "" && h.SecurityLayer != "DEFAULT" {
			security = strings.ToLower(h.SecurityLayer)
		}
		content = fmt.Sprintf("%s://%s@%s:%d?security=%s&sni=%s&fp=%s#%s",
			proto, sampleUUID, h.Address, h.Port, security, h.Sni, h.Fingerprint, h.Remark)
	}

	headers := map[string]string{
		"profile-web-page-url":  "https://panel.domain.com/sub/preview",
		"subscription-userinfo": "upload=0; download=104857600; total=107374182400; expire=1790000000",
		"Content-Type":          contentType,
	}

	return &PreviewResponse{
		ClientType:     clientType,
		ContentType:    contentType,
		Headers:        headers,
		PreviewContent: content,
	}, nil
}
