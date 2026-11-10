package subscription

import (
	"encoding/json"
	"errors"

	"gorm.io/gorm"

	"gowave/internal/database"
	"gowave/internal/subtpl"
	"gowave/internal/subtpl/ojson"
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

// loadSettings returns the subscription settings, cached between writes.
// Callers must not modify the result.
func (h *Handler) loadSettings() (*subSettings, error) {
	snap := h.cache.snapshot()
	if snap == nil {
		return h.readSettings()
	}
	return snap.settings.get(h.readSettings)
}

// readSettings reads the subscription settings row.
func (h *Handler) readSettings() (*subSettings, error) {
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
	squad, err := h.externalSquad(*squadUUID)
	if err != nil || squad == nil {
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

// externalSquad returns the squad row, nil when it does not exist.
func (h *Handler) externalSquad(squadUUID string) (*database.ExternalSquad, error) {
	read := func() (*database.ExternalSquad, error) {
		var squad database.ExternalSquad
		err := h.db.Where("uuid = ?", squadUUID).First(&squad).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		return &squad, nil
	}
	snap := h.cache.snapshot()
	if snap == nil {
		return read()
	}
	return keyed[*database.ExternalSquad](&snap.squads, squadUUID).get(read)
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

type templateKey struct{ first, second string }

// squadTemplateName ports GetCachedTemplateNameQuery.
func (h *Handler) squadTemplateName(squadUUID, templateType string) (string, bool) {
	if templateType == "XRAY_BASE64" {
		return "", false
	}
	read := func() (string, error) { return h.readSquadTemplateName(squadUUID, templateType) }
	var name string
	var err error
	if snap := h.cache.snapshot(); snap == nil {
		name, err = read()
	} else {
		name, err = keyed[string](&snap.squadTemplates, templateKey{squadUUID, templateType}).get(read)
	}
	return name, err == nil && name != ""
}

// readSquadTemplateName returns "" when the squad has no template of the
// type; the error is a database failure.
func (h *Handler) readSquadTemplateName(squadUUID, templateType string) (string, error) {
	var names []*string
	err := h.db.Table("external_squads_templates AS est").
		Select("st.name").
		Joins("LEFT JOIN subscription_templates AS st ON st.uuid = est.template_uuid").
		Where("est.external_squad_uuid = ? AND est.template_type = ?", squadUUID, templateType).
		Limit(1).Pluck("st.name", &names).Error
	if err != nil {
		return "", err
	}
	if len(names) == 0 || names[0] == nil {
		return "", nil
	}
	return *names[0], nil
}

// templateResult keeps a template lookup, including a missing or invalid
// template, which is cached like a found one.
type templateResult struct {
	value *ojson.Object
	err   error
}

// templates ports getCachedTemplateByType: the named template of a type,
// else the one named "Default". Callers must not modify the result.
func (h *Handler) templates(templateType, name string) (*ojson.Object, error) {
	if name == "" {
		name = "Default"
	}
	read := func() (templateResult, error) { return h.readTemplate(templateType, name) }
	var result templateResult
	var err error
	if snap := h.cache.snapshot(); snap == nil {
		result, err = read()
	} else {
		result, err = keyed[templateResult](&snap.templates, templateKey{templateType, name}).get(read)
	}
	if err != nil {
		return nil, err
	}
	return result.value, result.err
}

// readTemplate's error is a database failure; a missing or unparsable
// template is reported in the result.
func (h *Handler) readTemplate(templateType, name string) (templateResult, error) {
	var row database.SubscriptionTemplate
	err := h.db.Where("name = ? AND template_type = ?", name, templateType).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = h.db.Where("name = ? AND template_type = ?", "Default", templateType).First(&row).Error
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return templateResult{err: err}, nil
	}
	if err != nil {
		return templateResult{}, err
	}
	value, err := parseTemplate(&row)
	return templateResult{value: value, err: err}, nil
}

func parseTemplate(row *database.SubscriptionTemplate) (*ojson.Object, error) {
	var value any
	var err error
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
		IncludeInSrrRules: []string(host.IncludeInSrrRules),
		Mapper:            mapper, RawInbound: rawInbound, InboundTag: inboundTag, XrayJSONTemplate: xrayTemplate,
	}
}

// hostsForUser ports GetHostsForUserQuery: hosts reachable through the
// user's internal squads (honouring each host's ALLOW_ONLY/EXCLUDE squad
// list), joined with their inbound and Xray JSON template. The host graph
// is cached; only the user's squads are read per request. The hosts are
// shared between requests and must not be modified.
func (h *Handler) hostsForUser(userID uint64, withDisabled, withHidden bool) ([]*subtpl.Host, error) {
	var index *hostIndex
	var err error
	if snap := h.cache.snapshot(); snap == nil {
		index, err = h.readHostIndex()
	} else {
		index, err = snap.hosts.get(h.readHostIndex)
	}
	if err != nil {
		return nil, err
	}
	var squads []string
	if err := h.db.Model(&database.InternalSquadMember{}).Where("user_id = ?", userID).
		Pluck("internal_squad_uuid", &squads).Error; err != nil {
		return nil, err
	}
	return index.forSquads(squads, withDisabled, withHidden), nil
}

// hostIndex is the host graph hostsForUser filters per user.
type hostIndex struct {
	// hosts are ordered by view_position and have a known inbound.
	hosts []indexedHost
	// squadInbounds maps an internal squad to its inbounds.
	squadInbounds map[string]map[string]bool
}

type indexedHost struct {
	host      *subtpl.Host
	inbound   string
	allowOnly bool
	// linked are the internal squads in the host's squad list.
	linked map[string]bool
}

// forSquads keeps the hosts whose inbound one of squads grants, where that
// squad is listed by an ALLOW_ONLY host or not listed by an EXCLUDE host.
func (index *hostIndex) forSquads(squads []string, withDisabled, withHidden bool) []*subtpl.Host {
	hosts := []*subtpl.Host{}
	for _, entry := range index.hosts {
		if entry.host.IsDisabled && !withDisabled || entry.host.IsHidden && !withHidden {
			continue
		}
		for _, squad := range squads {
			if index.squadInbounds[squad][entry.inbound] && entry.linked[squad] == entry.allowOnly {
				hosts = append(hosts, entry.host)
				break
			}
		}
	}
	return hosts
}

// readHostIndex loads every host with its inbound and Xray JSON template,
// parsing each inbound and template once.
func (h *Handler) readHostIndex() (*hostIndex, error) {
	// A NULL squad mode never matches upstream's comparison.
	var rows []database.Host
	if err := h.db.Where("internal_squads_mode IS NOT NULL").Order("view_position ASC").Find(&rows).Error; err != nil {
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
	type parsedInbound struct {
		raw any
		tag string
	}
	inbounds := map[string]parsedInbound{}
	if len(inboundUUIDs) > 0 {
		var list []database.ConfigProfileInbound
		if err := h.db.Select("uuid", "raw_inbound", "tag").Where("uuid IN ?", inboundUUIDs).Find(&list).Error; err != nil {
			return nil, err
		}
		for _, inbound := range list {
			inbounds[inbound.UUID] = parsedInbound{raw: parseJSONColumn(inbound.RawInbound), tag: inbound.Tag}
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
	var squadInbounds []database.InternalSquadInbound
	if err := h.db.Select("internal_squad_uuid", "inbound_uuid").Find(&squadInbounds).Error; err != nil {
		return nil, err
	}
	var links []database.InternalSquadHostLink
	if err := h.db.Select("host_uuid", "squad_uuid").Find(&links).Error; err != nil {
		return nil, err
	}
	index := &hostIndex{squadInbounds: map[string]map[string]bool{}}
	for _, row := range squadInbounds {
		if index.squadInbounds[row.InternalSquadUUID] == nil {
			index.squadInbounds[row.InternalSquadUUID] = map[string]bool{}
		}
		index.squadInbounds[row.InternalSquadUUID][row.InboundUUID] = true
	}
	linked := map[string]map[string]bool{}
	for _, link := range links {
		if linked[link.HostUUID] == nil {
			linked[link.HostUUID] = map[string]bool{}
		}
		linked[link.HostUUID][link.SquadUUID] = true
	}
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
		index.hosts = append(index.hosts, indexedHost{
			host:    templateHost(row, inbound.raw, inbound.tag, template),
			inbound: *row.ConfigProfileInboundUUID, allowOnly: row.InternalSquadsMode == "ALLOW_ONLY", linked: linked[row.UUID],
		})
	}
	return index, nil
}
