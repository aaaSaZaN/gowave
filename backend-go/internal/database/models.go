package database

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type StringArray []string

func (a StringArray) Value() (driver.Value, error) {
	if len(a) == 0 {
		return "{}", nil
	}
	var elements []string
	for _, s := range a {
		s = strings.ReplaceAll(s, "\\", "\\\\")
		s = strings.ReplaceAll(s, "\"", "\\\"")
		elements = append(elements, "\""+s+"\"")
	}
	return "{" + strings.Join(elements, ",") + "}", nil
}

func (a *StringArray) Scan(src interface{}) error {
	if src == nil {
		*a = []string{}
		return nil
	}
	var str string
	switch v := src.(type) {
	case []string:
		*a = append((*a)[:0], v...)
		return nil
	case string:
		str = v
	case []byte:
		str = string(v)
	default:
		return fmt.Errorf("unsupported type for StringArray: %T", src)
	}

	str = strings.TrimSpace(str)
	if str == "" || str == "{}" || str == "[]" {
		*a = []string{}
		return nil
	}

	if strings.HasPrefix(str, "[") && strings.HasSuffix(str, "]") {
		var list []string
		if err := json.Unmarshal([]byte(str), &list); err == nil {
			*a = list
			return nil
		}
	}

	if strings.HasPrefix(str, "{") && strings.HasSuffix(str, "}") {
		inner := str[1 : len(str)-1]
		if inner == "" {
			*a = []string{}
			return nil
		}
		var res []string
		var cur strings.Builder
		inQuotes := false
		escaped := false
		for _, r := range inner {
			if escaped {
				cur.WriteRune(r)
				escaped = false
				continue
			}
			if r == '\\' {
				escaped = true
				continue
			}
			if r == '"' {
				inQuotes = !inQuotes
				continue
			}
			if r == ',' && !inQuotes {
				res = append(res, cur.String())
				cur.Reset()
				continue
			}
			cur.WriteRune(r)
		}
		res = append(res, cur.String())
		*a = res
		return nil
	}

	*a = []string{str}
	return nil
}

type Admin struct {
	UUID         string    `gorm:"primaryKey;type:varchar(64)" json:"uuid"`
	Username     string    `gorm:"uniqueIndex;type:varchar(64)" json:"username"`
	PasswordHash string    `gorm:"type:varchar(255)" json:"-"`
	Role         string    `gorm:"type:varchar(32);default:'ADMIN'"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

func (Admin) TableName() string {
	return "admin"
}

type User struct {
	Revision               uint64     `gorm:"default:1" json:"-"`
	ID                     uint64     `gorm:"primaryKey;autoIncrement;column:id" json:"id"`
	ShortUUID              string     `gorm:"uniqueIndex;type:varchar(64);column:short_uuid" json:"shortUuid"`
	Username               string     `gorm:"uniqueIndex;type:varchar(64);column:username" json:"username"`
	Status                 string     `gorm:"type:varchar(16);default:'ACTIVE';column:status" json:"status"`
	TrafficLimitBytes      uint64     `gorm:"default:0;column:traffic_limit_bytes" json:"trafficLimitBytes"`
	TrafficLimitStrategy   string     `gorm:"type:varchar(32);default:'NO_RESET';column:traffic_limit_strategy" json:"trafficLimitStrategy"`
	ExpireAt               time.Time  `gorm:"column:expire_at;index" json:"expireAt"`
	LastTrafficResetAt     *time.Time `gorm:"column:last_traffic_reset_at" json:"lastTrafficResetAt,omitempty"`
	SubRevokedAt           *time.Time `gorm:"column:sub_revoked_at" json:"subRevokedAt,omitempty"`
	TrojanPassword         string     `gorm:"type:varchar(64);column:trojan_password" json:"trojanPassword"`
	VlessUUID              string     `gorm:"type:varchar(64);column:vless_uuid" json:"vlessUuid"`
	SsPassword             string     `gorm:"type:varchar(64);column:ss_password" json:"ssPassword"`
	Description            string     `gorm:"type:text;column:description" json:"description"`
	Tag                    string     `gorm:"type:varchar(64);column:tag" json:"tag"`
	TelegramID             *int64     `gorm:"column:telegram_id" json:"telegramId,omitempty"`
	Email                  string     `gorm:"type:varchar(128);column:email" json:"email"`
	HWIDDeviceLimit        *int       `gorm:"type:integer;column:hwid_device_limit" json:"hwidDeviceLimit,omitempty"`
	ExternalSquadUUID      *string    `gorm:"type:varchar(64);column:external_squad_uuid" json:"externalSquadUuid,omitempty"`
	LastTriggeredThreshold int        `gorm:"default:0;column:last_triggered_threshold" json:"lastTriggeredThreshold"`
	CreatedAt              time.Time  `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt              time.Time  `gorm:"column:updated_at" json:"updatedAt"`

	Traffic              *UserTraffic    `gorm:"foreignKey:ID;constraint:OnDelete:CASCADE" json:"traffic,omitempty"`
	ExternalSquad        *ExternalSquad  `gorm:"foreignKey:ExternalSquadUUID;references:UUID;constraint:OnDelete:SET NULL" json:"-"`
	ActiveInternalSquads []InternalSquad `gorm:"-" json:"activeInternalSquads"`
}

func (User) TableName() string {
	return "users"
}

type UserTraffic struct {
	ID                       uint64     `gorm:"primaryKey" json:"id"`
	UsedTrafficBytes         uint64     `gorm:"default:0" json:"usedTrafficBytes"`
	LifetimeUsedTrafficBytes uint64     `gorm:"default:0" json:"lifetimeUsedTrafficBytes"`
	OnlineAt                 *time.Time `json:"onlineAt,omitempty"`
	LastConnectedNodeUUID    *string    `gorm:"type:varchar(64)" json:"lastConnectedNodeUuid,omitempty"`
	FirstConnectedAt         *time.Time `json:"firstConnectedAt,omitempty"`
	LastConnectedNode        *Node      `gorm:"foreignKey:LastConnectedNodeUUID;references:UUID;constraint:OnDelete:SET NULL" json:"-"`
}

func (UserTraffic) TableName() string {
	return "user_traffic"
}

type Node struct {
	ID                        uint64         `gorm:"uniqueIndex;autoIncrement" json:"id"`
	UUID                      string         `gorm:"primaryKey;uniqueIndex;type:varchar(64)" json:"uuid"`
	Name                      string         `gorm:"uniqueIndex;type:varchar(64)" json:"name"`
	Address                   string         `gorm:"uniqueIndex;type:varchar(255)" json:"address"`
	Port                      *int           `gorm:"default:443" json:"port"`
	ProxyURL                  *string        `gorm:"type:varchar(255)" json:"proxyUrl"`
	IsConnected               bool           `gorm:"default:false" json:"isConnected"`
	IsConnecting              bool           `gorm:"default:false" json:"isConnecting"`
	IsDisabled                bool           `gorm:"default:false" json:"isDisabled"`
	CountryCode               string         `gorm:"type:varchar(8);default:'XX'" json:"countryCode"`
	TrafficLimitBytes         uint64         `gorm:"default:0" json:"trafficLimitBytes"`
	TrafficUsedBytes          uint64         `gorm:"default:0" json:"trafficUsedBytes"`
	IsTrafficTrackingActive   bool           `gorm:"default:false" json:"isTrafficTrackingActive"`
	TrafficResetDay           int            `gorm:"default:1" json:"trafficResetDay"`
	NotifyPercent             int            `gorm:"default:80" json:"notifyPercent"`
	ViewPosition              int            `gorm:"default:1" json:"viewPosition"`
	ConsumptionMultiplier     int64          `gorm:"default:1000000000" json:"consumptionMultiplier"`
	NodeConsumptionMultiplier int64          `gorm:"default:1000000000" json:"nodeConsumptionMultiplier"`
	Tags                      StringArray    `gorm:"type:text[];default:'{}'" json:"tags"`
	IntegrationUUIDs          StringArray    `gorm:"type:uuid[];default:'{}'" json:"integrationUuids"`
	IPs                       string         `gorm:"type:text;default:'[]'"`
	ActiveConfigProfileUUID   *string        `gorm:"type:varchar(64)" json:"activeConfigProfileUuid"`
	ProviderUUID              *string        `gorm:"type:varchar(64)" json:"providerUuid"`
	ActivePluginUUID          *string        `gorm:"type:varchar(64)" json:"activePluginUuid"`
	Note                      *string        `gorm:"type:text" json:"note"`
	LastStatusChange          *time.Time     `json:"lastStatusChange,omitempty"`
	LastStatusMessage         *string        `gorm:"type:text" json:"lastStatusMessage"`
	XrayConfigError           *string        `gorm:"type:text" json:"-"`
	CreatedAt                 time.Time      `json:"createdAt"`
	UpdatedAt                 time.Time      `json:"updatedAt"`
	ActiveConfigProfile       *ConfigProfile `gorm:"foreignKey:ActiveConfigProfileUUID;references:UUID;constraint:OnDelete:SET NULL" json:"-"`
	Provider                  *InfraProvider `gorm:"foreignKey:ProviderUUID;references:UUID;constraint:OnDelete:SET NULL" json:"-"`
	ActivePlugin              *NodePlugin    `gorm:"foreignKey:ActivePluginUUID;references:UUID;constraint:OnDelete:SET NULL" json:"-"`
}

func (Node) TableName() string {
	return "nodes"
}

type Integration struct {
	UUID        string    `gorm:"primaryKey;type:varchar(64)" json:"uuid"`
	Name        string    `gorm:"uniqueIndex;type:varchar(30)" json:"name"`
	Description *string   `gorm:"type:varchar(255)" json:"description,omitempty"`
	Config      string    `gorm:"type:text" json:"config"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

func (Integration) TableName() string { return "integrations" }

type NodesUserUsageHistory struct {
	NodeID     uint64    `gorm:"primaryKey;column:node_id" json:"nodeId"`
	CreatedAt  time.Time `gorm:"primaryKey;column:created_at;index:idx_nodes_user_usage_history_user_created,priority:2" json:"createdAt"`
	UserID     uint64    `gorm:"primaryKey;column:user_id;index:idx_nodes_user_usage_history_user_created,priority:1" json:"userId"`
	TotalBytes uint64    `gorm:"column:total_bytes" json:"totalBytes"`
	UpdatedAt  time.Time `gorm:"column:updated_at" json:"updatedAt"`
	Node       Node      `gorm:"foreignKey:NodeID;references:ID;constraint:OnDelete:CASCADE" json:"-"`
	User       User      `gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:CASCADE" json:"-"`
}

func (NodesUserUsageHistory) TableName() string { return "nodes_user_usage_history" }

type NodesUsageHistory struct {
	NodeUUID      string    `gorm:"primaryKey;column:node_uuid;type:varchar(64)" json:"nodeUuid"`
	CreatedAt     time.Time `gorm:"primaryKey;column:created_at" json:"createdAt"`
	DownloadBytes uint64    `gorm:"column:download_bytes" json:"downloadBytes"`
	UploadBytes   uint64    `gorm:"column:upload_bytes" json:"uploadBytes"`
	TotalBytes    uint64    `gorm:"column:total_bytes" json:"totalBytes"`
	UpdatedAt     time.Time `gorm:"column:updated_at" json:"updatedAt"`
	Node          Node      `gorm:"foreignKey:NodeUUID;references:UUID;constraint:OnDelete:CASCADE" json:"-"`
}

func (NodesUsageHistory) TableName() string { return "nodes_usage_history" }

type Host struct {
	UUID                         string                `gorm:"primaryKey;type:varchar(64)" json:"uuid"`
	ViewPosition                 int                   `gorm:"default:1" json:"viewPosition"`
	Remark                       string                `gorm:"type:varchar(128)" json:"remark"`
	Address                      string                `gorm:"type:varchar(255)" json:"address"`
	Port                         int                   `gorm:"default:443" json:"port"`
	Path                         string                `gorm:"type:varchar(255)" json:"path"`
	Sni                          string                `gorm:"type:varchar(255)" json:"sni"`
	Host                         string                `gorm:"type:varchar(255)" json:"host"`
	Alpn                         string                `gorm:"type:varchar(64)" json:"alpn"`
	Fingerprint                  string                `gorm:"type:varchar(64)" json:"fingerprint"`
	SecurityLayer                string                `gorm:"type:varchar(32);default:'DEFAULT'" json:"securityLayer"`
	XhttpExtraParams             string                `gorm:"type:text" json:"xhttpExtraParams"`
	MuxParams                    string                `gorm:"type:text" json:"muxParams"`
	SockoptParams                string                `gorm:"type:text" json:"sockoptParams"`
	FinalMask                    string                `gorm:"type:text" json:"finalMask"`
	IsDisabled                   bool                  `gorm:"default:false" json:"isDisabled"`
	ServerDescription            string                `gorm:"type:varchar(255)" json:"serverDescription"`
	VlessRouteId                 *int                  `json:"vlessRouteId"`
	PinnedPeerCertSha256         string                `gorm:"type:varchar(255)" json:"pinnedPeerCertSha256"`
	VerifyPeerCertByName         string                `gorm:"type:varchar(255)" json:"verifyPeerCertByName"`
	ShuffleHost                  bool                  `gorm:"default:false" json:"shuffleHost"`
	MihomoX25519                 bool                  `gorm:"default:false" json:"mihomoX25519"`
	MihomoIpVersion              string                `gorm:"type:varchar(32)" json:"mihomoIpVersion"`
	XrayJsonTemplateUUID         *string               `gorm:"type:varchar(64)" json:"xrayJsonTemplateUuid"`
	KeepSniBlank                 bool                  `gorm:"default:false" json:"keepSniBlank"`
	ExcludeFromSubscriptionTypes StringArray           `gorm:"type:text[];default:'{}'" json:"excludeFromSubscriptionTypes"`
	Mapper                       string                `gorm:"type:text;default:'{}'" json:"mapper"`
	InternalSquadsMode           string                `gorm:"type:varchar(32);default:'EXCLUDE'" json:"internalSquadsMode"`
	InternalSquads               string                `gorm:"-" json:"internalSquads"`
	Tags                         StringArray           `gorm:"type:text[];default:'{}'" json:"tags"`
	Nodes                        string                `gorm:"-" json:"nodes"`
	IsHidden                     bool                  `gorm:"default:false" json:"isHidden"`
	OverrideSniFromAddress       bool                  `gorm:"default:false" json:"overrideSniFromAddress"`
	ConfigProfileUUID            *string               `gorm:"type:varchar(64)" json:"configProfileUuid"`
	ConfigProfileInboundUUID     *string               `gorm:"type:varchar(64)" json:"configProfileInboundUuid"`
	CreatedAt                    time.Time             `gorm:"-" json:"-"`
	UpdatedAt                    time.Time             `gorm:"-" json:"-"`
	ConfigProfile                *ConfigProfile        `gorm:"foreignKey:ConfigProfileUUID;references:UUID;constraint:OnDelete:SET NULL" json:"-"`
	ConfigProfileInbound         *ConfigProfileInbound `gorm:"foreignKey:ConfigProfileInboundUUID;references:UUID;constraint:OnDelete:SET NULL" json:"-"`
	XrayTemplate                 *SubscriptionTemplate `gorm:"foreignKey:XrayJsonTemplateUUID;references:UUID;constraint:OnDelete:SET NULL" json:"-"`
}

func (Host) TableName() string {
	return "hosts"
}

type HostsToNode struct {
	HostUUID string `gorm:"primaryKey;column:host_uuid;type:varchar(64)" json:"hostUuid"`
	NodeUUID string `gorm:"primaryKey;column:node_uuid;type:varchar(64)" json:"nodeUuid"`
	Host     Host   `gorm:"foreignKey:HostUUID;references:UUID;constraint:OnDelete:CASCADE" json:"-"`
	Node     Node   `gorm:"foreignKey:NodeUUID;references:UUID;constraint:OnDelete:CASCADE" json:"-"`
}

func (HostsToNode) TableName() string { return "hosts_to_nodes" }

type InternalSquadHostLink struct {
	HostUUID  string        `gorm:"primaryKey;column:host_uuid;type:varchar(64)" json:"hostUuid"`
	SquadUUID string        `gorm:"primaryKey;column:squad_uuid;type:varchar(64)" json:"squadUuid"`
	Host      Host          `gorm:"foreignKey:HostUUID;references:UUID;constraint:OnDelete:CASCADE" json:"-"`
	Squad     InternalSquad `gorm:"foreignKey:SquadUUID;references:UUID;constraint:OnDelete:CASCADE" json:"-"`
}

func (InternalSquadHostLink) TableName() string { return "internal_squad_host_links" }

type ConfigProfile struct {
	UUID         string                 `gorm:"primaryKey;type:varchar(64)" json:"uuid"`
	ViewPosition int                    `gorm:"default:1" json:"viewPosition"`
	Name         string                 `gorm:"uniqueIndex;type:varchar(64)" json:"name"`
	Tags         StringArray            `gorm:"type:text[];default:'{}'"`
	Config       string                 `gorm:"type:text" json:"config"`
	Inbounds     []ConfigProfileInbound `gorm:"foreignKey:ProfileUUID" json:"inbounds,omitempty"`
	Nodes        []Node                 `gorm:"foreignKey:ActiveConfigProfileUUID;references:UUID" json:"-"`
	Hosts        []Host                 `gorm:"foreignKey:ConfigProfileUUID;references:UUID" json:"-"`
	CreatedAt    time.Time              `json:"createdAt"`
	UpdatedAt    time.Time              `json:"updatedAt"`
}

func (ConfigProfile) TableName() string {
	return "config_profiles"
}

type ConfigProfileInbound struct {
	UUID        string                         `gorm:"primaryKey;type:varchar(64)" json:"uuid"`
	ProfileUUID string                         `gorm:"type:varchar(64);index" json:"profileUuid"`
	Tag         string                         `gorm:"uniqueIndex;type:varchar(64)" json:"tag"`
	Type        string                         `gorm:"type:varchar(32)" json:"type"`
	Network     *string                        `gorm:"type:varchar(32)" json:"network"`
	Security    *string                        `gorm:"type:varchar(32)" json:"security"`
	Port        *int                           `json:"port"`
	RawInbound  string                         `gorm:"type:text" json:"rawInbound"`
	Profile     ConfigProfile                  `gorm:"foreignKey:ProfileUUID;references:UUID;constraint:OnDelete:CASCADE" json:"-"`
	Hosts       []Host                         `gorm:"foreignKey:ConfigProfileInboundUUID;references:UUID" json:"-"`
	NodeLinks   []ConfigProfileInboundsToNodes `gorm:"foreignKey:ConfigProfileInboundUUID;references:UUID;constraint:OnDelete:CASCADE" json:"-"`
	SquadLinks  []InternalSquadInbound         `gorm:"foreignKey:InboundUUID;references:UUID;constraint:OnDelete:CASCADE" json:"-"`
}

func (ConfigProfileInbound) TableName() string {
	return "config_profile_inbounds"
}

type ConfigProfileInboundsToNodes struct {
	ConfigProfileInboundUUID string               `gorm:"primaryKey;column:config_profile_inbound_uuid;type:varchar(64)" json:"configProfileInboundUuid"`
	NodeUUID                 string               `gorm:"primaryKey;column:node_uuid;type:varchar(64)" json:"nodeUuid"`
	Inbound                  ConfigProfileInbound `gorm:"foreignKey:ConfigProfileInboundUUID;references:UUID;constraint:OnDelete:CASCADE" json:"-"`
	Node                     Node                 `gorm:"foreignKey:NodeUUID;references:UUID;constraint:OnDelete:CASCADE" json:"-"`
}

func (ConfigProfileInboundsToNodes) TableName() string {
	return "config_profile_inbounds_to_nodes"
}

type PanelSetting struct {
	ID                  uint   `gorm:"primaryKey" json:"id"`
	Title               string `gorm:"type:varchar(128);default:'GoWave'"`
	LogoURL             string `gorm:"type:varchar(255)" json:"logoUrl"`
	IsLoginAllowed      bool   `gorm:"default:true" json:"isLoginAllowed"`
	IsRegisterAllowed   bool   `gorm:"default:false" json:"isRegisterAllowed"`
	PasswordAuthEnabled bool   `gorm:"default:true" json:"passwordAuthEnabled"`
	PasskeySettings     string `gorm:"type:text;default:'{}'"`
	OAuth2Settings      string `gorm:"column:oauth2_settings;type:text;default:'{}'"`
	PasswordSettings    string `gorm:"type:text;default:'{}'"`
	BrandingSettings    string `gorm:"type:text;default:'{}'"`
}

func (PanelSetting) TableName() string {
	return "remnawave_settings"
}

type ApiToken struct {
	UUID      string      `gorm:"primaryKey;type:varchar(64)" json:"uuid"`
	Name      string      `gorm:"type:text" json:"name"`
	ExpireAt  time.Time   `gorm:"column:expire_at" json:"expireAt"`
	Token     string      `gorm:"-" json:"token"`
	Scopes    StringArray `gorm:"type:text[];default:'{}'" json:"scopes"`
	CreatedAt time.Time   `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt time.Time   `gorm:"column:updated_at" json:"updatedAt"`
}

func (ApiToken) TableName() string {
	return "api_tokens"
}

type SubscriptionTemplate struct {
	UUID         string      `gorm:"primaryKey;type:varchar(64)" json:"uuid"`
	ViewPosition int         `gorm:"default:0" json:"viewPosition"`
	Name         string      `gorm:"uniqueIndex:idx_subscription_templates_type_name,priority:2;type:varchar(255)" json:"name"`
	Tags         StringArray `gorm:"type:text[];default:'{}'"`
	TemplateType string      `gorm:"uniqueIndex:idx_subscription_templates_type_name,priority:1;type:varchar(32)" json:"templateType"`
	TemplateYaml string      `gorm:"type:text" json:"templateYaml"`
	TemplateJson string      `gorm:"type:text" json:"templateJson"`
	CreatedAt    time.Time   `json:"createdAt"`
	UpdatedAt    time.Time   `json:"updatedAt"`
	HostLinks    []Host      `gorm:"foreignKey:XrayJsonTemplateUUID;references:UUID" json:"-"`
}

func (SubscriptionTemplate) TableName() string {
	return "subscription_templates"
}

type SubscriptionPageConfig struct {
	UUID           string          `gorm:"primaryKey;type:varchar(64)" json:"uuid"`
	ViewPosition   int             `gorm:"default:0" json:"viewPosition"`
	Name           string          `gorm:"uniqueIndex;type:varchar(255)" json:"name"`
	Tags           StringArray     `gorm:"type:text[];default:'{}'"`
	Config         string          `gorm:"type:text;default:'{}'"`
	CreatedAt      time.Time       `json:"createdAt"`
	UpdatedAt      time.Time       `json:"updatedAt"`
	ExternalSquads []ExternalSquad `gorm:"foreignKey:SubpageConfigUUID;references:UUID" json:"-"`
}

func (SubscriptionPageConfig) TableName() string {
	return "subscription_page_config"
}

type ConfigProfileSnippet struct {
	Name      string    `gorm:"primaryKey;type:varchar(255)" json:"name"`
	Snippet   string    `gorm:"type:text;default:'{}'"`
	CreatedAt time.Time `json:"createdAt"`
}

func (ConfigProfileSnippet) TableName() string {
	return "config_profile_snippets"
}

type SharedList struct {
	Name      string    `gorm:"primaryKey;type:varchar(255)" json:"name"`
	Config    string    `gorm:"type:text;default:'{}'"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (SharedList) TableName() string {
	return "shared_lists"
}

type NodePlugin struct {
	UUID         string      `gorm:"primaryKey;type:varchar(64)" json:"uuid"`
	ViewPosition int         `gorm:"default:0" json:"viewPosition"`
	Name         string      `gorm:"type:varchar(255)" json:"name"`
	Tags         StringArray `gorm:"type:text[];default:'{}'"`
	PluginConfig string      `gorm:"type:text;default:'{}'"`
	CreatedAt    time.Time   `json:"createdAt"`
	UpdatedAt    time.Time   `json:"updatedAt"`
}

func (NodePlugin) TableName() string {
	return "node_plugin"
}

type InternalSquad struct {
	UUID         string      `gorm:"primaryKey;type:varchar(64)" json:"uuid"`
	ViewPosition int         `gorm:"default:0" json:"viewPosition"`
	Name         string      `gorm:"uniqueIndex;type:varchar(64)" json:"name"`
	Tags         StringArray `gorm:"type:text[];default:'{}'"`
	CreatedAt    time.Time   `json:"createdAt"`
	UpdatedAt    time.Time   `json:"updatedAt"`
}

func (InternalSquad) TableName() string {
	return "internal_squads"
}

type InternalSquadMember struct {
	InternalSquadUUID string        `gorm:"primaryKey;column:internal_squad_uuid;type:varchar(64)" json:"internalSquadUuid"`
	UserID            uint64        `gorm:"primaryKey;column:user_id;index" json:"userId"`
	Squad             InternalSquad `gorm:"foreignKey:InternalSquadUUID;references:UUID;constraint:OnDelete:CASCADE" json:"-"`
	User              User          `gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:CASCADE" json:"-"`
}

func (InternalSquadMember) TableName() string {
	return "internal_squad_members"
}

type InternalSquadInbound struct {
	InternalSquadUUID string               `gorm:"primaryKey;column:internal_squad_uuid;type:varchar(64)" json:"internalSquadUuid"`
	InboundUUID       string               `gorm:"primaryKey;column:inbound_uuid;type:varchar(64)" json:"inboundUuid"`
	Squad             InternalSquad        `gorm:"foreignKey:InternalSquadUUID;references:UUID;constraint:OnDelete:CASCADE" json:"-"`
	Inbound           ConfigProfileInbound `gorm:"foreignKey:InboundUUID;references:UUID;constraint:OnDelete:CASCADE" json:"-"`
}

func (InternalSquadInbound) TableName() string {
	return "internal_squad_inbounds"
}

type ExternalSquadTemplate struct {
	ExternalSquadUUID string               `gorm:"primaryKey;column:external_squad_uuid;type:varchar(64)" json:"externalSquadUuid"`
	TemplateType      string               `gorm:"primaryKey;column:template_type;type:varchar(32)" json:"templateType"`
	TemplateUUID      string               `gorm:"column:template_uuid;type:varchar(64)" json:"templateUuid"`
	Squad             ExternalSquad        `gorm:"foreignKey:ExternalSquadUUID;references:UUID;constraint:OnDelete:CASCADE" json:"-"`
	Template          SubscriptionTemplate `gorm:"foreignKey:TemplateUUID;references:UUID;constraint:OnDelete:CASCADE" json:"-"`
}

func (ExternalSquadTemplate) TableName() string {
	return "external_squads_templates"
}

type ExternalSquad struct {
	UUID                  string                  `gorm:"primaryKey;type:varchar(64)" json:"uuid"`
	ViewPosition          int                     `gorm:"default:0" json:"viewPosition"`
	Name                  string                  `gorm:"uniqueIndex;type:varchar(64)" json:"name"`
	Tags                  StringArray             `gorm:"type:text[];default:'{}'"`
	SubscriptionSettings  string                  `gorm:"type:text;default:'{}'"`
	HostOverrides         string                  `gorm:"type:text;default:'{}'"`
	ResponseHeadersAdd    string                  `gorm:"type:text;default:'{}'"`
	ResponseHeadersRemove StringArray             `gorm:"type:text[];default:'{}'"`
	HwidSettings          string                  `gorm:"type:text;default:'{}'"`
	CustomRemarks         string                  `gorm:"type:text;default:'{}'"`
	SubpageConfigUUID     *string                 `gorm:"type:varchar(64)" json:"subpageConfigUuid"`
	CreatedAt             time.Time               `json:"createdAt"`
	UpdatedAt             time.Time               `json:"updatedAt"`
	SubpageConfig         *SubscriptionPageConfig `gorm:"foreignKey:SubpageConfigUUID;references:UUID;constraint:OnDelete:SET NULL" json:"-"`
}

func (ExternalSquad) TableName() string {
	return "external_squads"
}

type InfraProvider struct {
	UUID         string                `gorm:"primaryKey;type:varchar(64)" json:"uuid"`
	Name         string                `gorm:"uniqueIndex;type:varchar(128)" json:"name"`
	FaviconLink  *string               `gorm:"type:varchar(255)" json:"faviconLink"`
	LoginURL     *string               `gorm:"type:varchar(255)" json:"loginUrl"`
	CreatedAt    time.Time             `json:"createdAt"`
	UpdatedAt    time.Time             `json:"updatedAt"`
	Nodes        []Node                `gorm:"foreignKey:ProviderUUID;references:UUID" json:"-"`
	BillingNodes []InfraBillingNode    `gorm:"foreignKey:ProviderUUID;references:UUID;constraint:OnDelete:CASCADE" json:"-"`
	BillingItems []InfraBillingHistory `gorm:"foreignKey:ProviderUUID;references:UUID;constraint:OnDelete:CASCADE" json:"-"`
}

func (InfraProvider) TableName() string {
	return "infra_providers"
}

type InfraBillingNode struct {
	UUID          string        `gorm:"primaryKey;type:varchar(64)" json:"uuid"`
	NodeUUID      *string       `gorm:"uniqueIndex:idx_infra_billing_nodes_node_provider,priority:1;type:varchar(64)" json:"nodeUuid"`
	Name          string        `gorm:"type:varchar(128)" json:"name"`
	ProviderUUID  string        `gorm:"uniqueIndex:idx_infra_billing_nodes_node_provider,priority:2;type:varchar(64)" json:"providerUuid"`
	NextBillingAt time.Time     `gorm:"index" json:"nextBillingAt"`
	CreatedAt     time.Time     `json:"createdAt"`
	UpdatedAt     time.Time     `json:"updatedAt"`
	Node          *Node         `gorm:"foreignKey:NodeUUID;references:UUID;constraint:OnDelete:CASCADE" json:"-"`
	Provider      InfraProvider `gorm:"foreignKey:ProviderUUID;references:UUID;constraint:OnDelete:CASCADE" json:"-"`
}

func (InfraBillingNode) TableName() string {
	return "infra_billing_nodes"
}

type InfraBillingHistory struct {
	UUID         string        `gorm:"primaryKey;type:varchar(64)" json:"uuid"`
	ProviderUUID string        `gorm:"type:varchar(64)" json:"providerUuid"`
	Amount       float64       `json:"amount"`
	BilledAt     time.Time     `json:"billedAt"`
	Provider     InfraProvider `gorm:"foreignKey:ProviderUUID;references:UUID;constraint:OnDelete:CASCADE" json:"-"`
}

func (InfraBillingHistory) TableName() string {
	return "infra_billing_history"
}

type HwidDevice struct {
	HWID        string    `gorm:"primaryKey;column:hwid;type:varchar(128)" json:"hwid"`
	UserID      uint64    `gorm:"primaryKey;index" json:"userId"`
	Platform    *string   `gorm:"type:varchar(64)" json:"platform"`
	OSVersion   *string   `gorm:"column:os_version;type:varchar(64)" json:"osVersion"`
	DeviceModel *string   `gorm:"type:varchar(128)" json:"deviceModel"`
	UserAgent   *string   `gorm:"type:text" json:"userAgent"`
	RequestIP   *string   `gorm:"type:varchar(64)" json:"requestIp"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	User        User      `gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:CASCADE" json:"-"`
}

func (HwidDevice) TableName() string {
	return "hwid_user_devices"
}

type UserSubscriptionRequestHistory struct {
	ID              uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID          uint64    `gorm:"index" json:"userId"`
	RequestIP       *string   `gorm:"type:varchar(64)" json:"requestIp"`
	UserAgent       *string   `gorm:"type:text" json:"userAgent"`
	SrrRuleName     *string   `gorm:"type:varchar(128)" json:"srrRuleName"`
	SrrResponseType string    `gorm:"type:varchar(64);default:'UNKNOWN'"`
	RequestAt       time.Time `json:"requestAt"`
	User            User      `gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:CASCADE" json:"-"`
}

func (UserSubscriptionRequestHistory) TableName() string {
	return "user_subscription_request_history"
}

type EntityMeta struct {
	EntityID   string `gorm:"primaryKey;type:varchar(64)" json:"entityId"`
	EntityType string `gorm:"primaryKey;type:varchar(32)" json:"entityType"`
	Metadata   string `gorm:"type:text;default:'{}'"`
}

func (EntityMeta) TableName() string {
	return "entity_meta"
}

type Keygen struct {
	UUID       string    `gorm:"primaryKey;type:varchar(64)" json:"uuid"`
	CACert     string    `gorm:"column:ca_cert;type:text" json:"caCert"`
	CAKey      string    `gorm:"column:ca_key;type:text" json:"caKey"`
	ClientCert string    `gorm:"type:text" json:"clientCert"`
	ClientKey  string    `gorm:"type:text" json:"clientKey"`
	PubKey     string    `gorm:"type:text" json:"pubKey"`
	PrivKey    string    `gorm:"type:text" json:"privKey"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

func (Keygen) TableName() string {
	return "keygen"
}

type Passkey struct {
	ID              string    `gorm:"primaryKey;type:varchar(255)" json:"id"`
	AdminUUID       string    `gorm:"type:varchar(64);index" json:"adminUuid"`
	PublicKey       []byte    `json:"-"`
	CredentialData  string    `gorm:"type:text" json:"-"`
	Counter         int64     `gorm:"default:0" json:"counter"`
	DeviceType      string    `gorm:"type:varchar(32);default:'singleDevice'"`
	BackedUp        bool      `gorm:"default:false" json:"backedUp"`
	Transports      string    `gorm:"type:varchar(255)" json:"transports"`
	PasskeyProvider string    `gorm:"type:varchar(255);default:'Unknown'"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
	Admin           Admin     `gorm:"foreignKey:AdminUUID;references:UUID;constraint:OnDelete:CASCADE" json:"-"`
}

// AuthChallenge also stores single-use OAuth state and backend-tool tickets.
type AuthChallenge struct {
	ID        string    `gorm:"primaryKey;type:varchar(255)"`
	Purpose   string    `gorm:"index;type:varchar(64)"`
	AdminUUID string    `gorm:"type:varchar(64)"`
	Data      string    `gorm:"type:text"`
	ExpiresAt time.Time `gorm:"index"`
	CreatedAt time.Time
}

type Job struct {
	ID          string     `gorm:"primaryKey;type:varchar(64)" json:"id"`
	Kind        string     `gorm:"index;type:varchar(64)" json:"kind"`
	EntityID    string     `gorm:"index;type:varchar(64)" json:"entityId"`
	Target      string     `gorm:"index;type:varchar(255)" json:"target"`
	Revision    uint64     `json:"revision"`
	Payload     string     `gorm:"type:text" json:"-"`
	Result      string     `gorm:"type:text" json:"-"`
	State       string     `gorm:"index;type:varchar(16);default:'pending'" json:"state"`
	Attempts    int        `json:"attempts"`
	AvailableAt time.Time  `gorm:"index" json:"availableAt"`
	LeaseUntil  *time.Time `gorm:"index" json:"leaseUntil"`
	LastError   string     `gorm:"type:text" json:"lastError"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

type ScheduledExecution struct {
	ID         string `gorm:"primaryKey;type:varchar(128)"`
	ExecutedAt time.Time
}

func (Passkey) TableName() string {
	return "passkeys"
}

type UserMeta struct {
	UserID   uint64 `gorm:"primaryKey;column:user_id" json:"userId"`
	Metadata string `gorm:"type:text" json:"metadata"`
	User     User   `gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:CASCADE" json:"-"`
}

func (UserMeta) TableName() string { return "user_meta" }

type NodeMeta struct {
	NodeID   uint64 `gorm:"primaryKey;column:node_id" json:"nodeId"`
	Metadata string `gorm:"type:text" json:"metadata"`
	Node     Node   `gorm:"foreignKey:NodeID;references:ID;constraint:OnDelete:CASCADE" json:"-"`
}

func (NodeMeta) TableName() string { return "node_meta" }

type TorrentBlockerReport struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID    uint64    `gorm:"column:user_id" json:"userId"`
	NodeID    uint64    `gorm:"column:node_id" json:"nodeId"`
	Report    string    `gorm:"type:text" json:"report"`
	CreatedAt time.Time `gorm:"column:created_at" json:"createdAt"`
	User      User      `gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:CASCADE" json:"-"`
	Node      Node      `gorm:"foreignKey:NodeID;references:ID;constraint:OnDelete:CASCADE" json:"-"`
}

func (TorrentBlockerReport) TableName() string { return "torrent_blocker_reports" }

type SubscriptionSetting struct {
	UUID                        string    `gorm:"primaryKey;type:varchar(64)" json:"uuid"`
	ServeJsonAtBaseSubscription bool      `gorm:"default:false" json:"serveJsonAtBaseSubscription"`
	IsShowCustomRemarks         bool      `gorm:"default:true" json:"isShowCustomRemarks"`
	CustomRemarks               string    `gorm:"type:text;default:'{}'" json:"customRemarks"`
	CustomResponseHeaders       string    `gorm:"type:text;default:'{}'" json:"customResponseHeaders"`
	RandomizeHosts              bool      `gorm:"default:false" json:"randomizeHosts"`
	ResponseRules               string    `gorm:"type:text;default:'{}'" json:"responseRules"`
	HwidSettings                string    `gorm:"type:text;default:'{}'" json:"hwidSettings"`
	CreatedAt                   time.Time `json:"createdAt"`
	UpdatedAt                   time.Time `json:"updatedAt"`
}

func (SubscriptionSetting) TableName() string {
	return "subscription_settings"
}
