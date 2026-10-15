package subscription

import (
	"encoding/json"
	"errors"

	"gorm.io/gorm"

	"remnawave-go/internal/database"
	"remnawave-go/internal/subtpl"
	"remnawave-go/internal/subtpl/ojson"
)

// hwidSettings is THwidSettings.
type hwidSettings struct {
	Enabled             bool    `json:"enabled"`
	FallbackDeviceLimit int     `json:"fallbackDeviceLimit"`
	MaxDevicesAnnounce  *string `json:"maxDevicesAnnounce"`
}

// subSettings is the part of SubscriptionSettingsEntity the public
// subscription path reads, after external squad overrides.
type subSettings struct {
	ServeJSONAtBaseSubscription bool
	IsShowCustomRemarks         bool
	RandomizeHosts              bool
	CustomRemarks               subtpl.CustomRemarks
	// CustomResponseHeaders keeps upstream's key order; nil when null.
	CustomResponseHeaders *ojson.Object
	Hwid                  hwidSettings
	// ResponseRules is nil when the column is null.
	ResponseRules *SRRConfig
}

func (s *subSettings) template() *subtpl.Settings {
	limit := s.Hwid.FallbackDeviceLimit
	return &subtpl.Settings{IsShowCustomRemarks: s.IsShowCustomRemarks, CustomRemarks: s.CustomRemarks, HwidFallbackDeviceLimit: &limit}
}

func jsonPresent(raw string) bool {
	return raw != "" && raw != "null"
}

// hasContent ports the helper of the same name: null, undefined and empty
// objects carry no override.
func hasContent(raw string) bool {
	if !jsonPresent(raw) {
		return false
	}
	value, err := ojson.Parse([]byte(raw))
	if err != nil || value == nil {
		return false
	}
	switch v := value.(type) {
	case *ojson.Object:
		return len(v.Keys()) > 0
	case []any:
		return len(v) > 0
	}
	return true
}

func parseHeaderObject(raw string) *ojson.Object {
	if !jsonPresent(raw) {
		return nil
	}
	value, err := ojson.Parse([]byte(raw))
	if err != nil {
		return nil
	}
	object, _ := value.(*ojson.Object)
	return object
}

// loadSettings reads the subscription settings row.
func (h *Handler) loadSettings() (*subSettings, error) {
	var row database.SubscriptionSetting
	if err := h.db.First(&row).Error; err != nil {
		return nil, err
	}
	settings := &subSettings{
		ServeJSONAtBaseSubscription: row.ServeJsonAtBaseSubscription,
		IsShowCustomRemarks:         row.IsShowCustomRemarks,
		RandomizeHosts:              row.RandomizeHosts,
		CustomResponseHeaders:       parseHeaderObject(row.CustomResponseHeaders),
	}
	if jsonPresent(row.CustomRemarks) {
		_ = json.Unmarshal([]byte(row.CustomRemarks), &settings.CustomRemarks)
	}
	if jsonPresent(row.HwidSettings) {
		_ = json.Unmarshal([]byte(row.HwidSettings), &settings.Hwid)
	}
	if jsonPresent(row.ResponseRules) {
		var rules SRRConfig
		if err := json.Unmarshal([]byte(row.ResponseRules), &rules); err == nil {
			settings.ResponseRules = &rules
		}
	}
	return settings, nil
}

// applySquadOverrides ports applyMaybeExternalSquadOverrides. The input is
// never modified.
func (h *Handler) applySquadOverrides(settings *subSettings, squadUUID *string) (*subSettings, *subtpl.HostOverrides) {
	if squadUUID == nil {
		return settings, nil
	}
	var squad database.ExternalSquad
	if err := h.db.Where("uuid = ?", *squadUUID).First(&squad).Error; err != nil {
		return settings, nil
	}
	patched := *settings
	if settings.CustomResponseHeaders != nil {
		patched.CustomResponseHeaders = ojson.Clone(settings.CustomResponseHeaders).(*ojson.Object)
	}
	var overrides *subtpl.HostOverrides
	if hasContent(squad.HostOverrides) {
		overrides = parseHostOverrides(squad.HostOverrides)
	}
	if hasContent(squad.SubscriptionSettings) {
		var values struct {
			ServeJSONAtBaseSubscription *bool `json:"serveJsonAtBaseSubscription"`
			IsShowCustomRemarks         *bool `json:"isShowCustomRemarks"`
			RandomizeHosts              *bool `json:"randomizeHosts"`
		}
		if json.Unmarshal([]byte(squad.SubscriptionSettings), &values) == nil {
			if values.ServeJSONAtBaseSubscription != nil {
				patched.ServeJSONAtBaseSubscription = *values.ServeJSONAtBaseSubscription
			}
			if values.IsShowCustomRemarks != nil {
				patched.IsShowCustomRemarks = *values.IsShowCustomRemarks
			}
			if values.RandomizeHosts != nil {
				patched.RandomizeHosts = *values.RandomizeHosts
			}
		}
	}
	if len(squad.ResponseHeadersRemove) > 0 {
		remove := map[string]bool{}
		for _, key := range squad.ResponseHeadersRemove {
			remove[key] = true
		}
		kept := ojson.New()
		if patched.CustomResponseHeaders != nil {
			for _, key := range patched.CustomResponseHeaders.Keys() {
				if !remove[key] {
					kept.Set(key, patched.CustomResponseHeaders.Value(key))
				}
			}
		}
		patched.CustomResponseHeaders = kept
	}
	if hasContent(squad.ResponseHeadersAdd) {
		merged := ojson.New()
		if patched.CustomResponseHeaders != nil {
			merged = ojson.Clone(patched.CustomResponseHeaders).(*ojson.Object)
		}
		if add := parseHeaderObject(squad.ResponseHeadersAdd); add != nil {
			for _, key := range add.Keys() {
				merged.Set(key, add.Value(key))
			}
		}
		patched.CustomResponseHeaders = merged
	}
	if hasContent(squad.HwidSettings) {
		var hwid hwidSettings
		if json.Unmarshal([]byte(squad.HwidSettings), &hwid) == nil {
			patched.Hwid = hwid
		}
	}
	if hasContent(squad.CustomRemarks) {
		var remarks subtpl.CustomRemarks
		if json.Unmarshal([]byte(squad.CustomRemarks), &remarks) == nil {
			patched.CustomRemarks = remarks
		}
	}
	return &patched, overrides
}

func parseHostOverrides(raw string) *subtpl.HostOverrides {
	var fields map[string]json.RawMessage
	if json.Unmarshal([]byte(raw), &fields) != nil {
		return nil
	}
	overrides := &subtpl.HostOverrides{}
	if value, ok := fields["vlessRouteId"]; ok {
		var route *int
		if json.Unmarshal(value, &route) == nil {
			overrides.VlessRouteID = &route
		}
	}
	if value, ok := fields["serverDescription"]; ok {
		var description *string
		if json.Unmarshal(value, &description) == nil {
			overrides.ServerDescription = &description
		}
	}
	return overrides
}

// squadTemplateName ports GetCachedTemplateNameQuery.
func (h *Handler) squadTemplateName(squadUUID, templateType string) (string, bool) {
	if templateType == "XRAY_BASE64" {
		return "", false
	}
	var names []*string
	err := h.db.Table("external_squads_templates AS est").
		Select("st.name").
		Joins("LEFT JOIN subscription_templates AS st ON st.uuid = est.template_uuid").
		Where("est.external_squad_uuid = ? AND est.template_type = ?", squadUUID, templateType).
		Limit(1).Pluck("st.name", &names).Error
	if err != nil || len(names) == 0 || names[0] == nil || *names[0] == "" {
		return "", false
	}
	return *names[0], true
}

// templates ports getCachedTemplateByType: the named template of a type,
// else the one named "Default".
func (h *Handler) templates(templateType, name string) (*ojson.Object, error) {
	if name == "" {
		name = "Default"
	}
	var row database.SubscriptionTemplate
	err := h.db.Where("name = ? AND template_type = ?", name, templateType).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = h.db.Where("name = ? AND template_type = ?", "Default", templateType).First(&row).Error
	}
	if err != nil {
		return nil, err
	}
	var value any
	switch row.TemplateType {
	case "MIHOMO", "STASH", "CLASH":
		value, err = subtpl.LoadYAML(row.TemplateYaml)
	default:
		if !jsonPresent(row.TemplateJson) {
			return nil, errors.New("template content is null")
		}
		value, err = ojson.Parse([]byte(row.TemplateJson))
	}
	if err != nil {
		return nil, err
	}
	object, ok := value.(*ojson.Object)
	if !ok {
		return nil, errors.New("template content is not an object")
	}
	return object, nil
}

func nullable(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

// templateUser builds the generator's view of a user.
func templateUser(user *database.User) *subtpl.User {
	var used, lifetime uint64
	if user.Traffic != nil {
		used, lifetime = user.Traffic.UsedTrafficBytes, user.Traffic.LifetimeUsedTrafficBytes
	}
	return &subtpl.User{
		ID: int64(user.ID), ShortUUID: user.ShortUUID, Username: user.Username, Status: user.Status,
		TrafficLimitBytes: int64(user.TrafficLimitBytes), TrafficLimitStrategy: user.TrafficLimitStrategy,
		UsedTrafficBytes: int64(used), LifetimeUsedTrafficBytes: int64(lifetime),
		ExpireAt: user.ExpireAt.UTC(), CreatedAt: user.CreatedAt.UTC(), LastTrafficResetAt: user.LastTrafficResetAt,
		Email: nullable(user.Email), TelegramID: user.TelegramID, Tag: nullable(user.Tag), Description: nullable(user.Description),
		HwidDeviceLimit: user.HWIDDeviceLimit, VlessUUID: user.VlessUUID, TrojanPassword: user.TrojanPassword, SsPassword: user.SsPassword,
	}
}

func parseJSONColumn(raw string) any {
	if !jsonPresent(raw) {
		return nil
	}
	value, err := ojson.Parse([]byte(raw))
	if err != nil {
		return nil
	}
	return value
}

var subscriptionTemplateTypes = map[string]bool{"XRAY_JSON": true, "XRAY_BASE64": true, "MIHOMO": true, "STASH": true, "CLASH": true, "SINGBOX": true}

func oneOf(value string, allowed ...string) *string {
	for _, candidate := range allowed {
		if value == candidate {
			return &value
		}
	}
	return nil
}

// templateHost converts a host row the way the hosts API reports it.
func templateHost(host *database.Host, rawInbound any, inboundTag string, xrayTemplate any) *subtpl.Host {
	tags := []string(host.Tags)
	if tags == nil {
		tags = []string{}
	}
	exclude := []string{}
	for _, kind := range host.ExcludeFromSubscriptionTypes {
		if subscriptionTemplateTypes[kind] {
			exclude = append(exclude, kind)
		}
	}
	mapper, _ := parseJSONColumn(host.Mapper).(*ojson.Object)
	if mapper == nil {
		mapper = ojson.New()
	}
	position := host.ViewPosition
	if position <= 0 {
		position = 1
	}
	securityLayer := host.SecurityLayer
	if securityLayer != "TLS" && securityLayer != "NONE" {
		securityLayer = "DEFAULT"
	}
	return &subtpl.Host{
		UUID: host.UUID, ViewPosition: position, Remark: host.Remark, Address: host.Address, Port: host.Port,
		Path: nullable(host.Path), SNI: nullable(host.Sni), Host: nullable(host.Host),
		ALPN:             oneOf(host.Alpn, "h3", "h2", "http/1.1", "h2,http/1.1", "h3,h2,http/1.1", "h3,h2"),
		Fingerprint:      nullable(host.Fingerprint),
		SecurityLayer:    securityLayer,
		XhttpExtraParams: parseJSONColumn(host.XhttpExtraParams), MuxParams: parseJSONColumn(host.MuxParams),
		SockoptParams: parseJSONColumn(host.SockoptParams), FinalMask: parseJSONColumn(host.FinalMask),
		IsDisabled: host.IsDisabled, ServerDescription: nullable(host.ServerDescription),
		PinnedPeerCertSha256: nullable(host.PinnedPeerCertSha256), VerifyPeerCertByName: nullable(host.VerifyPeerCertByName),
		MihomoIPVersion: oneOf(host.MihomoIpVersion, "dual", "ipv4", "ipv6", "ipv4-prefer", "ipv6-prefer"),
		Tags:            tags, IsHidden: host.IsHidden, OverrideSniFromAddress: host.OverrideSniFromAddress,
		KeepSniBlank: host.KeepSniBlank, VlessRouteID: host.VlessRouteId, ShuffleHost: host.ShuffleHost,
		MihomoX25519: host.MihomoX25519, ConfigProfileUUID: host.ConfigProfileUUID,
		ConfigProfileInboundUUID: host.ConfigProfileInboundUUID, ExcludeFromSubscriptionTypes: exclude,
		Mapper: mapper, RawInbound: rawInbound, InboundTag: inboundTag, XrayJSONTemplate: xrayTemplate,
	}
}

// hostsForUser ports GetHostsForUserQuery: hosts reachable through the
// user's internal squads (honouring each host's ALLOW_ONLY/EXCLUDE squad
// list), joined with their inbound and Xray JSON template.
func (h *Handler) hostsForUser(userID uint64, withDisabled, withHidden bool) ([]*subtpl.Host, error) {
	query := h.db.Model(&database.Host{}).Where(`EXISTS (
		SELECT 1 FROM internal_squad_inbounds AS isi
		JOIN internal_squad_members AS ism ON ism.internal_squad_uuid = isi.internal_squad_uuid
		WHERE isi.inbound_uuid = hosts.config_profile_inbound_uuid AND ism.user_id = ?
		AND (EXISTS (
			SELECT 1 FROM internal_squad_host_links AS link
			WHERE link.host_uuid = hosts.uuid AND link.squad_uuid = isi.internal_squad_uuid
		)) = (hosts.internal_squads_mode = 'ALLOW_ONLY')
	)`, userID)
	if !withDisabled {
		query = query.Where("hosts.is_disabled = ?", false)
	}
	if !withHidden {
		query = query.Where("hosts.is_hidden = ?", false)
	}
	var rows []database.Host
	if err := query.Order("hosts.view_position ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	inboundUUIDs, templateUUIDs := []string{}, []string{}
	for _, row := range rows {
		if row.ConfigProfileInboundUUID != nil {
			inboundUUIDs = append(inboundUUIDs, *row.ConfigProfileInboundUUID)
		}
		if row.XrayJsonTemplateUUID != nil {
			templateUUIDs = append(templateUUIDs, *row.XrayJsonTemplateUUID)
		}
	}
	inbounds := map[string]database.ConfigProfileInbound{}
	if len(inboundUUIDs) > 0 {
		var list []database.ConfigProfileInbound
		if err := h.db.Select("uuid", "raw_inbound", "tag").Where("uuid IN ?", inboundUUIDs).Find(&list).Error; err != nil {
			return nil, err
		}
		for _, inbound := range list {
			inbounds[inbound.UUID] = inbound
		}
	}
	templates := map[string]any{}
	if len(templateUUIDs) > 0 {
		var list []database.SubscriptionTemplate
		if err := h.db.Select("uuid", "template_json").Where("uuid IN ?", templateUUIDs).Find(&list).Error; err != nil {
			return nil, err
		}
		for _, template := range list {
			templates[template.UUID] = parseJSONColumn(template.TemplateJson)
		}
	}
	hosts := make([]*subtpl.Host, 0, len(rows))
	for i := range rows {
		row := &rows[i]
		if row.ConfigProfileInboundUUID == nil {
			continue
		}
		inbound, ok := inbounds[*row.ConfigProfileInboundUUID]
		if !ok {
			continue
		}
		var template any
		if row.XrayJsonTemplateUUID != nil {
			template = templates[*row.XrayJsonTemplateUUID]
		}
		hosts = append(hosts, templateHost(row, parseJSONColumn(inbound.RawInbound), inbound.Tag, template))
	}
	return hosts, nil
}
