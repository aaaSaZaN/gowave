package subtpl

import (
	"strings"

	"remnawave-go/internal/subtpl/ojson"
)

// parseResolvedFromRemark ports parseResolvedProxyConfigFromRemark: a custom
// remark that is a serialized ResolvedProxyConfig becomes that host.
func parseResolvedFromRemark(remark string) *Resolved {
	if !strings.HasPrefix(remark, `{"f`) {
		return nil
	}
	parsed, err := ojson.Parse([]byte(remark))
	if err != nil {
		return nil
	}
	object, ok := parsed.(*ojson.Object)
	if !ok {
		return nil
	}
	return resolvedFromObject(object)
}

func optString(o any, path ...string) *string { return asStringPtr(get(o, path...)) }

func num(o any, path ...string) float64 {
	value, _ := get(o, path...).(float64)
	return value
}

func strList(value any) []string {
	list, _ := value.([]any)
	out := []string{}
	for _, item := range list {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func resolvedFromObject(o *ojson.Object) *Resolved {
	obj := func(path ...string) *ojson.Object {
		value, _ := get(o, path...).(*ojson.Object)
		return value
	}
	r := &Resolved{
		FinalRemark:       asString(o.Value("finalRemark")),
		Address:           asString(o.Value("address")),
		Port:              int(num(o, "port")),
		FinalMask:         obj("streamOverrides", "finalMask"),
		Sockopt:           obj("streamOverrides", "sockopt"),
		Mux:               obj("mux"),
		ShuffleHost:       ojson.Truthy(get(o, "clientOverrides", "shuffleHost")),
		MihomoX25519:      ojson.Truthy(get(o, "clientOverrides", "mihomoX25519")),
		MihomoIPVersion:   optString(o, "clientOverrides", "mihomoIpVersion"),
		ServerDescription: optString(o, "clientOverrides", "serverDescription"),
		XrayJSONTemplate:  get(o, "clientOverrides", "xrayJsonTemplate"),
		Mapper:            obj("clientOverrides", "mapper"),
		Protocol:          asString(o.Value("protocol")),
		Security:          asString(o.Value("security")),
		Transport:         asString(o.Value("transport")),
	}
	if r.Mapper == nil {
		r.Mapper = ojson.New()
	}
	meta := obj("metadata")
	r.Meta = Metadata{
		UUID: asString(get(meta, "uuid")), Tags: strList(get(meta, "tags")),
		ExcludeFromSubscriptionTypes: strList(get(meta, "excludeFromSubscriptionTypes")),
		InboundTag:                   asString(get(meta, "inboundTag")),
		IsHidden:                     ojson.Truthy(get(meta, "isHidden")),
		Remark:                       asString(get(meta, "remark")),
		RawInbound:                   get(meta, "rawInbound"),
	}
	p := func(key string) any { return get(o, "protocolOptions", key) }
	r.ID, r.Encryption, r.Flow = asString(p("id")), asString(p("encryption")), asString(p("flow"))
	r.Password, r.Method = asString(p("password")), asString(p("method"))
	r.UoT, r.UoTVersion, r.Version = ojson.Truthy(p("uot")), num(o, "protocolOptions", "uotVersion"), num(o, "protocolOptions", "version")
	s := func(key string) any { return get(o, "securityOptions", key) }
	switch r.Security {
	case "tls":
		r.TLS = &TLSOptions{ALPN: asString(s("alpn")), EnableSessionResumption: definedOrUndefined(o, "securityOptions", "enableSessionResumption"),
			Fingerprint: asString(s("fingerprint")), ServerName: asString(s("serverName")), EchConfigList: s("echConfigList"),
			EchForceQuery: s("echForceQuery"), EchSockopt: obj("securityOptions", "echSockopt"),
			PinnedPeerCertSha256: optString(o, "securityOptions", "pinnedPeerCertSha256"),
			VerifyPeerCertByName: optString(o, "securityOptions", "verifyPeerCertByName"), CipherSuites: s("cipherSuites")}
	case "reality":
		r.Reality = &RealityOptions{Fingerprint: asString(s("fingerprint")), PublicKey: asString(s("publicKey")), ShortID: s("shortId"),
			ServerName: asString(s("serverName")), SpiderX: s("spiderX"), Mldsa65Verify: optString(o, "securityOptions", "mldsa65Verify")}
	}
	t := func(key string) any { return get(o, "transportOptions", key) }
	switch r.Transport {
	case "tcp":
		r.TCP = &TCPOptions{Header: t("header")}
	case "xhttp":
		extra, _ := t("extra").(*ojson.Object)
		r.XHTTP = &XHTTPOptions{Path: asStringPtr(t("path")), Host: asString(t("host")), Mode: t("mode"), Extra: extra}
	case "ws":
		r.WS = &WSOptions{Host: asString(t("host")), Path: asStringPtr(t("path")), Headers: t("headers"), HeartbeatPeriod: t("heartbeatPeriod")}
	case "httpupgrade":
		r.HTTPUp = &HTTPUpgradeOptions{Path: asStringPtr(t("path")), Host: asString(t("host")), Headers: t("headers")}
	case "grpc":
		r.GRPC = &GRPCOptions{Authority: asString(t("authority")), ServiceName: asStringPtr(t("serviceName")), MultiMode: ojson.Truthy(t("multiMode"))}
	case "kcp":
		r.KCP = &KCPOptions{ClientMtu: t("clientMtu"), ClientTti: t("clientTti"), Congestion: t("congestion")}
	case "hysteria":
		r.Hysteria = &HysteriaOptions{Version: num(o, "transportOptions", "version"), Auth: asString(t("auth"))}
	default:
		r.TCP = &TCPOptions{}
	}
	return r
}

func definedOrUndefined(o any, path ...string) any {
	if value, ok := getDefined(o, path...); ok {
		return value
	}
	return ojson.Undefined
}
