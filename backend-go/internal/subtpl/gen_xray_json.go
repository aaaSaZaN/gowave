package subtpl

import (
	"regexp"

	"gowave/internal/subtpl/js"
	"gowave/internal/subtpl/ojson"
)

// XrayJSONParams mirrors IGenerateConfigParams.
type XrayJSONParams struct {
	Hosts                      []*Resolved
	IsExtendedClient           bool
	Template                   *ojson.Object
	IgnoreHostXrayJSONTemplate bool
}

// GenerateXrayJSON ports XrayJsonGeneratorService.generateConfig.
func GenerateXrayJSON(params XrayJSONParams) string {
	configs := []any{}
	for _, host := range params.Hosts {
		if host.Meta.IsHidden || contains(host.Meta.ExcludeFromSubscriptionTypes, "XRAY_JSON") {
			continue
		}
		base := params.Template
		if !params.IgnoreHostXrayJSONTemplate {
			if template, ok := host.XrayJSONTemplate.(*ojson.Object); ok && template != nil {
				base = template
			}
		}
		if ojson.Truthy(base.Value("remnawave")) {
			if injected := applyRemnawaveInjector(base, host, params.Hosts, params.IsExtendedClient); injected != nil {
				configs = append(configs, injected)
			}
			continue
		}
		outbounds := []any{buildXrayOutbound(host, "proxy")}
		if existing, ok := base.Value("outbounds").([]any); ok {
			outbounds = append(outbounds, existing...)
		}
		config := ojson.New().Spread(base).Set("outbounds", outbounds).Set("remarks", host.FinalRemark).Set("meta", xrayMeta(host, params.IsExtendedClient))
		configs = append(configs, config)
	}
	return ojson.Stringify(configs)
}

func xrayMeta(host *Resolved, isExtendedClient bool) any {
	if isExtendedClient && host.ServerDescription != nil && *host.ServerDescription != "" {
		return ojson.New("serverDescription", js.DecodeBase64String(*host.ServerDescription))
	}
	return ojson.Undefined
}

func buildXrayOutbound(host *Resolved, tag string) any {
	outbound := ojson.New("tag", tag, "protocol", host.Protocol, "settings", xrayProtocolSettings(host), "streamSettings", xrayStreamSettings(host))
	if host.Mux != nil {
		mux := ojson.New()
		for _, key := range host.Mux.Keys() {
			if key != "smux" {
				mux.Set(key, host.Mux.Value(key))
			}
		}
		if mux.Len() > 0 {
			outbound.Set("mux", mux)
		}
	}
	return applyHostMapper(outbound, host.mapperOps("xrayJson"), host, false)
}

func xrayProtocolSettings(host *Resolved) *ojson.Object {
	port := float64(host.Port)
	switch host.Protocol {
	case "vless":
		encryption := host.Encryption
		if encryption == "" {
			encryption = "none"
		}
		return ojson.New("vnext", []any{ojson.New("address", host.Address, "port", port, "users", []any{
			ojson.New("id", host.ID, "encryption", encryption, "flow", host.Flow),
		})})
	case "trojan":
		return ojson.New("servers", []any{ojson.New("address", host.Address, "port", port, "password", host.Password)})
	case "hysteria":
		return ojson.New("address", host.Address, "port", port, "version", 2.0)
	case "shadowsocks":
		return ojson.New("servers", []any{ojson.New("address", host.Address, "port", port, "password", host.Password, "method", host.Method, "uot", host.UoT, "UoTVersion", host.UoTVersion)})
	}
	return nil
}

// spreadHeaders is { ...headers }: a copy of an object, {} otherwise.
func spreadHeaders(headers any) *ojson.Object {
	copy := ojson.New()
	if o, ok := headers.(*ojson.Object); ok {
		copy.Spread(o)
	}
	return copy
}

func xrayStreamSettings(host *Resolved) *ojson.Object {
	stream := ojson.New("network", host.Transport)
	switch host.Transport {
	case "ws":
		ws := ojson.New("path", ptrAny(host.WS.Path), "host", host.WS.Host, "headers", spreadHeaders(host.WS.Headers))
		if host.WS.HeartbeatPeriod != nil {
			ws.Set("heartbeatPeriod", host.WS.HeartbeatPeriod)
		}
		stream.Set("wsSettings", ws)
	case "httpupgrade":
		stream.Set("httpupgradeSettings", ojson.New("path", ptrAny(host.HTTPUp.Path), "host", host.HTTPUp.Host, "headers", spreadHeaders(host.HTTPUp.Headers)))
	case "tcp":
		tcp := ojson.New()
		if ojson.Truthy(host.TCP.Header) {
			tcp.Set("header", host.TCP.Header)
		}
		stream.Set("tcpSettings", tcp)
	case "xhttp":
		x := ojson.New("mode", host.XHTTP.Mode, "host", host.XHTTP.Host)
		if host.XHTTP.Path != nil && *host.XHTTP.Path != "" {
			x.Set("path", *host.XHTTP.Path)
		}
		if host.XHTTP.Extra != nil {
			x.Set("extra", host.XHTTP.Extra)
		}
		stream.Set("xhttpSettings", x)
	case "grpc":
		stream.Set("grpcSettings", ojson.New("serviceName", ptrAny(host.GRPC.ServiceName), "authority", host.GRPC.Authority, "multiMode", host.GRPC.MultiMode))
	case "kcp":
		stream.Set("kcpSettings", ojson.New("mtu", host.KCP.ClientMtu, "tti", host.KCP.ClientTti, "congestion", host.KCP.Congestion))
	case "hysteria":
		stream.Set("hysteriaSettings", ojson.New("version", 2.0, "auth", host.Hysteria.Auth))
	}
	switch host.Security {
	case "tls":
		o := host.TLS
		tls := ojson.New("serverName", o.ServerName, "enableSessionResumption", o.EnableSessionResumption)
		if o.Fingerprint != "" {
			tls.Set("fingerprint", o.Fingerprint)
		}
		if o.ALPN != "" {
			tls.Set("alpn", splitAny(o.ALPN, ","))
		}
		setTruthy(tls, "pinnedPeerCertSha256", ptrAny(o.PinnedPeerCertSha256))
		setTruthy(tls, "verifyPeerCertByName", ptrAny(o.VerifyPeerCertByName))
		setTruthy(tls, "echForceQuery", o.EchForceQuery)
		setTruthy(tls, "echConfigList", o.EchConfigList)
		if o.EchSockopt != nil {
			tls.Set("echSockopt", o.EchSockopt)
		}
		setTruthy(tls, "cipherSuites", o.CipherSuites)
		stream.Set("security", "tls").Set("tlsSettings", tls)
	case "reality":
		o := host.Reality
		reality := ojson.New("serverName", o.ServerName)
		setTruthy(reality, "publicKey", o.PublicKey)
		setTruthy(reality, "mldsa65Verify", ptrAny(o.Mldsa65Verify))
		setTruthy(reality, "shortId", o.ShortID)
		setTruthy(reality, "spiderX", o.SpiderX)
		if o.Fingerprint != "" {
			reality.Set("fingerprint", o.Fingerprint)
		}
		stream.Set("security", "reality").Set("realitySettings", reality)
	case "none":
		stream.Set("security", "none")
	}
	if host.Sockopt != nil {
		stream.Set("sockopt", host.Sockopt)
	}
	if host.FinalMask != nil {
		stream.Set("finalmask", host.FinalMask)
	}
	return stream
}

func splitAny(s, sep string) []any {
	parts := []any{}
	start := 0
	for i := 0; i+len(sep) <= len(s); i++ {
		if s[i:i+len(sep)] == sep {
			parts = append(parts, s[start:i])
			start = i + len(sep)
		}
	}
	return append(parts, s[start:])
}

func buildTaggedOutbounds(hosts []*Resolved, entry *ojson.Object) []any {
	var out []any
	switch {
	case ojson.Truthy(entry.Value("useHostRemarkAsTag")):
		for _, h := range hosts {
			out = append(out, buildXrayOutbound(h, h.FinalRemark))
		}
	case ojson.Truthy(entry.Value("useHostTagAsTag")):
		for _, h := range hosts {
			tag := h.FinalRemark
			if len(h.Meta.Tags) > 0 && h.Meta.Tags[0] != "" {
				tag = h.Meta.Tags[0]
			}
			out = append(out, buildXrayOutbound(h, tag))
		}
	default:
		prefix := "proxy"
		if p, ok := entry.Get("tagPrefix"); ok && p != nil && p != ojson.Undefined {
			prefix = jsString(p)
		}
		for i, h := range hosts {
			tag := prefix
			if i > 0 {
				tag = prefix + "-" + itoa(i+1)
			}
			out = append(out, buildXrayOutbound(h, tag))
		}
	}
	return out
}

func resolveInjectedHosts(selector *ojson.Object, selectFrom any, host *Resolved, all []*Resolved) []*Resolved {
	source := "HIDDEN"
	if selectFrom != nil && selectFrom != ojson.Undefined {
		source = jsString(selectFrom)
	}
	var candidates []*Resolved
	for _, h := range all {
		if h.Meta.UUID == host.Meta.UUID {
			continue
		}
		switch source {
		case "ALL":
			candidates = append(candidates, h)
		case "HIDDEN":
			if h.Meta.IsHidden {
				candidates = append(candidates, h)
			}
		case "NOT_HIDDEN":
			if !h.Meta.IsHidden {
				candidates = append(candidates, h)
			}
		}
	}
	var result []*Resolved
	switch selector.Value("type") {
	case "uuids":
		values, _ := selector.Value("values").([]any)
		for _, value := range values {
			for _, h := range candidates {
				if h.Meta.UUID == value {
					result = append(result, h)
					break
				}
			}
		}
	case "remarkRegex":
		if re, err := regexp.Compile(asString(selector.Value("pattern"))); err == nil {
			for _, h := range candidates {
				if re.MatchString(h.FinalRemark) {
					result = append(result, h)
				}
			}
		}
	case "sameTagAsRecipient":
		for _, h := range candidates {
			if len(h.Meta.Tags) > 0 && len(host.Meta.Tags) > 0 && sharesTag(h.Meta.Tags, host.Meta.Tags) {
				result = append(result, h)
			}
		}
	case "tagRegex":
		if re, err := regexp.Compile(asString(selector.Value("pattern"))); err == nil {
			for _, h := range candidates {
				for _, tag := range h.Meta.Tags {
					if re.MatchString(tag) {
						result = append(result, h)
						break
					}
				}
			}
		}
	}
	return result
}

func sharesTag(a, b []string) bool {
	for _, x := range a {
		if contains(b, x) {
			return true
		}
	}
	return false
}

func applyRemnawaveInjector(base *ojson.Object, host *Resolved, all []*Resolved, isExtendedClient bool) any {
	injector, _ := base.Value("remnawave").(*ojson.Object)
	if injector == nil {
		return nil
	}
	if !ojson.Truthy(injector.Value("injectHosts")) && !ojson.Truthy(injector.Value("addVirtualHostAsOutbound")) {
		return nil
	}
	template := ojson.New()
	for _, key := range base.Keys() {
		if key != "remnawave" {
			template.Set(key, base.Value(key))
		}
	}
	var injected []any
	if ojson.Truthy(injector.Value("addVirtualHostAsOutbound")) {
		injected = append(injected, buildXrayOutbound(host, "proxy"))
	}
	entries, _ := injector.Value("injectHosts").([]any)
	for _, raw := range entries {
		entry, ok := raw.(*ojson.Object)
		if !ok {
			continue
		}
		selector, _ := entry.Value("selector").(*ojson.Object)
		if selector == nil {
			continue
		}
		injected = append(injected, buildTaggedOutbounds(resolveInjectedHosts(selector, entry.Value("selectFrom"), host, all), entry)...)
	}
	existing, _ := template.Value("outbounds").([]any)
	outbounds := append(append([]any{}, injected...), existing...)
	config := ojson.New().Spread(template).Set("outbounds", outbounds).Set("remarks", host.FinalRemark)
	if isExtendedClient && host.ServerDescription != nil && *host.ServerDescription != "" {
		config.Set("meta", ojson.New("serverDescription", js.DecodeBase64String(*host.ServerDescription)))
	}
	return config
}
