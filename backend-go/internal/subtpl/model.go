// Package subtpl ports upstream's subscription-template module
// (src/modules/subscription-template at upstream/backend 3.4.5): proxy
// resolution, the five client generators, the remark template engine and the
// host mapper. scripts/subscription-diff compares it with the TypeScript.
package subtpl

import (
	"time"

	"gowave/internal/subtpl/ojson"
)

// Host mirrors HostWithRawInbound: a host row plus its inbound's raw JSON.
type Host struct {
	UUID                         string
	ViewPosition                 int
	Remark                       string
	Address                      string
	Port                         int
	Path                         *string
	SNI                          *string
	Host                         *string
	ALPN                         *string
	Fingerprint                  *string
	SecurityLayer                string
	XhttpExtraParams             any
	MuxParams                    any
	SockoptParams                any
	FinalMask                    any
	IsDisabled                   bool
	ServerDescription            *string
	PinnedPeerCertSha256         *string
	VerifyPeerCertByName         *string
	MihomoIPVersion              *string
	Tags                         []string
	IsHidden                     bool
	OverrideSniFromAddress       bool
	KeepSniBlank                 bool
	VlessRouteID                 *int
	ShuffleHost                  bool
	MihomoX25519                 bool
	ConfigProfileUUID            *string
	ConfigProfileInboundUUID     *string
	ExcludeFromSubscriptionTypes []string
	// Mapper is THostMapper ({} when unset).
	Mapper           *ojson.Object
	RawInbound       any
	InboundTag       string
	XrayJSONTemplate any
}

// User carries the user fields the generators and remark templates read.
type User struct {
	ID                       int64
	ShortUUID                string
	Username                 string
	Status                   string
	TrafficLimitBytes        int64
	TrafficLimitStrategy     string
	UsedTrafficBytes         int64
	LifetimeUsedTrafficBytes int64
	ExpireAt                 time.Time
	CreatedAt                time.Time
	LastTrafficResetAt       *time.Time
	Email                    *string
	TelegramID               *int64
	Tag                      *string
	Description              *string
	HwidDeviceLimit          *int
	VlessUUID                string
	TrojanPassword           string
	SsPassword               string
}

// CustomRemarks is subscriptionSettings.customRemarks.
type CustomRemarks struct {
	ExpiredUsers           []string `json:"expiredUsers"`
	LimitedUsers           []string `json:"limitedUsers"`
	DisabledUsers          []string `json:"disabledUsers"`
	EmptyHosts             []string `json:"emptyHosts"`
	HWIDMaxDevicesExceeded []string `json:"HWIDMaxDevicesExceeded"`
	HWIDNotSupported       []string `json:"HWIDNotSupported"`
}

// Settings is the part of SubscriptionSettingsEntity proxy resolution reads.
type Settings struct {
	IsShowCustomRemarks     bool
	CustomRemarks           CustomRemarks
	HwidFallbackDeviceLimit *int
}

// HostOverrides is ExternalSquadEntity.hostOverrides; nil fields are unset.
type HostOverrides struct {
	VlessRouteID      **int
	ServerDescription **string
}

// FallbackOptions mirrors IResolveProxyConfigOptions.fallbackOptions.
type FallbackOptions struct {
	ShowHwidMaxDeviceRemarks    bool
	ShowHwidNotSupportedRemarks bool
	RespondWithRemarks          []string
}

// Resolved is ResolvedProxyConfig. Nullable strings are *string.
type Resolved struct {
	FinalRemark string
	Address     string
	Port        int

	FinalMask *ojson.Object
	Sockopt   *ojson.Object
	Mux       *ojson.Object

	ShuffleHost       bool
	MihomoX25519      bool
	MihomoIPVersion   *string
	ServerDescription *string
	XrayJSONTemplate  any
	Mapper            *ojson.Object

	Meta Metadata

	Protocol string // vless | trojan | shadowsocks | hysteria
	// VLESS
	ID         string
	Encryption string
	Flow       string
	// Trojan / Shadowsocks
	Password   string
	Method     string
	UoT        bool
	UoTVersion float64
	// Hysteria protocol version
	Version float64

	Security string // tls | reality | none
	TLS      *TLSOptions
	Reality  *RealityOptions

	Transport string // tcp | xhttp | ws | httpupgrade | grpc | kcp | hysteria
	TCP       *TCPOptions
	XHTTP     *XHTTPOptions
	WS        *WSOptions
	HTTPUp    *HTTPUpgradeOptions
	GRPC      *GRPCOptions
	KCP       *KCPOptions
	Hysteria  *HysteriaOptions
}

// Metadata is IProxyEntryMetadata.
type Metadata struct {
	UUID                         string
	Tags                         []string
	ExcludeFromSubscriptionTypes []string
	InboundTag                   string
	ConfigProfileUUID            *string
	ConfigProfileInboundUUID     *string
	IsDisabled                   bool
	IsHidden                     bool
	ViewPosition                 int
	Remark                       string
	VlessRouteID                 *int
	RawInbound                   any
}

type TLSOptions struct {
	ALPN                    string
	EnableSessionResumption any // bool; Undefined when absent from a JSON remark
	Fingerprint             string
	ServerName              string
	EchConfigList           any
	EchForceQuery           any
	EchSockopt              *ojson.Object
	PinnedPeerCertSha256    *string
	VerifyPeerCertByName    *string
	CipherSuites            any
}

type RealityOptions struct {
	Fingerprint   string
	PublicKey     string
	ShortID       any
	ServerName    string
	SpiderX       any
	Mldsa65Verify *string
}

type TCPOptions struct{ Header any } // nil | object
type XHTTPOptions struct {
	Path  *string
	Host  string
	Mode  any
	Extra *ojson.Object
}
type WSOptions struct {
	Host            string
	Path            *string
	Headers         any
	HeartbeatPeriod any
}
type HTTPUpgradeOptions struct {
	Path    *string
	Host    string
	Headers any
}
type GRPCOptions struct {
	Authority   string
	ServiceName *string
	MultiMode   bool
}
type KCPOptions struct {
	ClientMtu  any
	ClientTti  any
	Congestion any
}
type HysteriaOptions struct {
	Version float64
	Auth    string
}

func str(p *string) any {
	if p == nil {
		return nil
	}
	return *p
}

func objOrNull(o *ojson.Object) any {
	if o == nil {
		return nil
	}
	return o
}

func intOrNull(p *int) any {
	if p == nil {
		return nil
	}
	return float64(*p)
}

func strings2any(list []string) []any {
	out := make([]any, len(list))
	for i, s := range list {
		out[i] = s
	}
	return out
}

// Object renders the resolved config with upstream's property order; the
// host mapper reads "$host." paths from it.
func (r *Resolved) Object() *ojson.Object {
	o := ojson.New(
		"finalRemark", r.FinalRemark,
		"address", r.Address,
		"port", float64(r.Port),
		"streamOverrides", ojson.New("finalMask", objOrNull(r.FinalMask), "sockopt", objOrNull(r.Sockopt)),
		"mux", objOrNull(r.Mux),
		"clientOverrides", ojson.New(
			"shuffleHost", r.ShuffleHost,
			"mihomoX25519", r.MihomoX25519,
			"mihomoIpVersion", str(r.MihomoIPVersion),
			"serverDescription", str(r.ServerDescription),
			"xrayJsonTemplate", r.XrayJSONTemplate,
			"mapper", objOrNull(r.Mapper),
		),
		"metadata", ojson.New(
			"uuid", r.Meta.UUID,
			"tags", strings2any(r.Meta.Tags),
			"excludeFromSubscriptionTypes", strings2any(r.Meta.ExcludeFromSubscriptionTypes),
			"inboundTag", r.Meta.InboundTag,
			"configProfileUuid", str(r.Meta.ConfigProfileUUID),
			"configProfileInboundUuid", str(r.Meta.ConfigProfileInboundUUID),
			"isDisabled", r.Meta.IsDisabled,
			"isHidden", r.Meta.IsHidden,
			"viewPosition", float64(r.Meta.ViewPosition),
			"remark", r.Meta.Remark,
			"vlessRouteId", intOrNull(r.Meta.VlessRouteID),
			"rawInbound", r.Meta.RawInbound,
		),
		"protocol", r.Protocol,
	)
	switch r.Protocol {
	case "vless":
		o.Set("protocolOptions", ojson.New("id", r.ID, "encryption", r.Encryption, "flow", r.Flow))
	case "trojan":
		o.Set("protocolOptions", ojson.New("password", r.Password))
	case "shadowsocks":
		o.Set("protocolOptions", ojson.New("method", r.Method, "password", r.Password, "uot", r.UoT, "uotVersion", r.UoTVersion))
	case "hysteria":
		o.Set("protocolOptions", ojson.New("version", r.Version))
	}
	o.Set("security", r.Security)
	switch r.Security {
	case "tls":
		t := r.TLS
		o.Set("securityOptions", ojson.New(
			"alpn", t.ALPN,
			"enableSessionResumption", t.EnableSessionResumption,
			"fingerprint", t.Fingerprint,
			"serverName", t.ServerName,
			"echConfigList", t.EchConfigList,
			"echForceQuery", t.EchForceQuery,
			"echSockopt", objOrNull(t.EchSockopt),
			"pinnedPeerCertSha256", str(t.PinnedPeerCertSha256),
			"verifyPeerCertByName", str(t.VerifyPeerCertByName),
			"cipherSuites", t.CipherSuites,
		))
	case "reality":
		t := r.Reality
		o.Set("securityOptions", ojson.New(
			"fingerprint", t.Fingerprint,
			"publicKey", t.PublicKey,
			"shortId", t.ShortID,
			"serverName", t.ServerName,
			"spiderX", t.SpiderX,
			"mldsa65Verify", str(t.Mldsa65Verify),
		))
	}
	o.Set("transport", r.Transport)
	switch r.Transport {
	case "tcp":
		o.Set("transportOptions", ojson.New("header", r.TCP.Header))
	case "xhttp":
		t := r.XHTTP
		o.Set("transportOptions", ojson.New("path", str(t.Path), "host", t.Host, "mode", t.Mode, "extra", objOrNull(t.Extra)))
	case "ws":
		t := r.WS
		o.Set("transportOptions", ojson.New("host", t.Host, "path", str(t.Path), "headers", t.Headers, "heartbeatPeriod", t.HeartbeatPeriod))
	case "httpupgrade":
		t := r.HTTPUp
		o.Set("transportOptions", ojson.New("path", str(t.Path), "host", t.Host, "headers", t.Headers))
	case "grpc":
		t := r.GRPC
		o.Set("transportOptions", ojson.New("authority", t.Authority, "serviceName", str(t.ServiceName), "multiMode", t.MultiMode))
	case "kcp":
		t := r.KCP
		o.Set("transportOptions", ojson.New("clientMtu", t.ClientMtu, "clientTti", t.ClientTti, "congestion", t.Congestion))
	case "hysteria":
		t := r.Hysteria
		o.Set("transportOptions", ojson.New("version", t.Version, "auth", t.Auth))
	}
	return o
}

// mapperOps returns the host mapper operations for a client ("base64",
// "xrayJson", "mihomo", "singbox").
func (r *Resolved) mapperOps(client string) []any {
	if r.Mapper == nil {
		return nil
	}
	ops, _ := r.Mapper.Value(client).([]any)
	return ops
}
