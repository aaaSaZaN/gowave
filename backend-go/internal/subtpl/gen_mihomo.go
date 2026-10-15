package subtpl

import (
	"math"
	"strings"

	"remnawave-go/internal/subtpl/js"
	"remnawave-go/internal/subtpl/ojson"
)

type fieldMapping struct {
	src, dst string
	asString bool
}

var xhttpFieldMap = []fieldMapping{
	{"noGRPCHeader", "no-grpc-header", false}, {"xPaddingBytes", "x-padding-bytes", true},
	{"xPaddingObfsMode", "x-padding-obfs-mode", false}, {"xPaddingKey", "x-padding-key", false},
	{"xPaddingHeader", "x-padding-header", false}, {"xPaddingPlacement", "x-padding-placement", false},
	{"xPaddingMethod", "x-padding-method", false}, {"uplinkHTTPMethod", "uplink-http-method", false},
	{"sessionIDPlacement", "session-placement", false}, {"sessionIDKey", "session-key", false},
	{"sessionIDTable", "session-table", false}, {"sessionIDLength", "session-length", true},
	{"seqPlacement", "seq-placement", false}, {"seqKey", "seq-key", false},
	{"uplinkDataPlacement", "uplink-data-placement", false}, {"uplinkDataKey", "uplink-data-key", false},
	{"uplinkChunkSize", "uplink-chunk-size", false}, {"scMaxEachPostBytes", "sc-max-each-post-bytes", false},
	{"scMinPostsIntervalMs", "sc-min-posts-interval-ms", false}, {"scStreamUpServerSecs", "sc-stream-up-server-secs", true},
}

var xmuxFieldMap = []fieldMapping{
	{"maxConnections", "max-connections", true}, {"maxConcurrency", "max-concurrency", true},
	{"cMaxReuseTimes", "c-max-reuse-times", true}, {"hMaxRequestTimes", "h-max-request-times", true},
	{"hMaxReusableSecs", "h-max-reusable-secs", true}, {"hKeepAlivePeriod", "h-keep-alive-period", false},
}

func applyFieldMap(source, target *ojson.Object, fields []fieldMapping) {
	for _, f := range fields {
		value, ok := source.Get(f.src)
		if !ok || value == ojson.Undefined {
			continue
		}
		if f.asString {
			target.Set(f.dst, jsString(value))
		} else {
			target.Set(f.dst, value)
		}
	}
}

// GenerateMihomo ports MihomoGeneratorService.generateConfig (isStash selects
// the STASH template and drops XHTTP).
func GenerateMihomo(hosts []*Resolved, template *ojson.Object, isStash, isExtendedClient bool) (string, error) {
	templateType := "MIHOMO"
	if isStash {
		templateType = "STASH"
	}
	includeHidden := get(template, "remnawave", "includeHiddenHosts") == true
	var proxies []any
	var remarks []string
	for _, host := range hosts {
		if !includeHidden && host.Meta.IsHidden {
			continue
		}
		if contains(host.Meta.ExcludeFromSubscriptionTypes, templateType) || host.Transport == "kcp" || (isStash && host.Transport == "xhttp") {
			continue
		}
		node := mihomoNode(host, isExtendedClient)
		if node == nil {
			continue
		}
		proxies = append(proxies, node)
		remarks = append(remarks, host.FinalRemark)
	}
	return renderClashFamily(template, proxies, remarks, true)
}

func mihomoNode(host *Resolved, isExtendedClient bool) any {
	var node *ojson.Object
	if host.Protocol == "hysteria" {
		node = mihomoHysteria2(host, isExtendedClient)
	} else {
		ipVersion := any(ojson.Undefined)
		if host.MihomoIPVersion != nil {
			ipVersion = *host.MihomoIPVersion
		}
		node = ojson.New("name", host.FinalRemark, "type", clashType(host.Protocol), "server", host.Address, "port", float64(host.Port),
			"network", clashNetwork(host), "udp", true, "ip-version", ipVersion)
		if !mihomoProtocolFields(node, host) {
			return nil
		}
		clashSecurityFields(node, host, true)
		mihomoTransportOpts(node, host)
		node.Set("client-fingerprint", clashFingerprint(host))
		if host.Mux != nil && host.Mux.Has("smux") {
			node.Set("smux", host.Mux.Value("smux"))
		}
		if isExtendedClient && host.ServerDescription != nil && *host.ServerDescription != "" {
			node.Set("serverDescription", js.DecodeBase64String(*host.ServerDescription))
		}
	}
	if node == nil {
		return nil
	}
	return applyHostMapper(node, host.mapperOps("mihomo"), host, false)
}

func clashType(protocol string) string {
	if protocol == "shadowsocks" {
		return "ss"
	}
	return protocol
}

func clashNetwork(host *Resolved) string {
	if host.Transport == "tcp" && get(host.TCP.Header, "type") == "http" {
		return "http"
	}
	if host.Transport == "httpupgrade" {
		return "ws"
	}
	return host.Transport
}

func mihomoProtocolFields(node *ojson.Object, host *Resolved) bool {
	switch host.Protocol {
	case "vless":
		node.Set("uuid", host.ID).Set("packet-encoding", "xudp")
		if host.Flow == "xtls-rprx-vision" {
			node.Set("flow", host.Flow)
		}
		if host.Encryption != "" && host.Encryption != "none" {
			node.Set("encryption", host.Encryption)
		}
		return true
	case "trojan":
		node.Set("password", host.Password)
		return true
	case "shadowsocks":
		node.Set("password", host.Password).Set("cipher", host.Method).Set("udp-over-tcp", host.UoT).Set("udp-over-tcp-version", host.UoTVersion)
		return true
	}
	return false
}

// clashSecurityFields covers both generators; reality applies to Mihomo only.
func clashSecurityFields(node *ojson.Object, host *Resolved, mihomo bool) {
	switch host.Security {
	case "tls":
		o := host.TLS
		node.Set("tls", true)
		if node.Value("type") == "trojan" {
			node.Set("sni", o.ServerName)
		} else {
			node.Set("servername", o.ServerName)
		}
		if o.ALPN != "" {
			node.Set("alpn", splitAny(o.ALPN, ","))
		}
		if o.PinnedPeerCertSha256 != nil && *o.PinnedPeerCertSha256 != "" && (!mihomo || node.Value("type") != "ss") {
			node.Set("skip-cert-verify", true)
		}
	case "reality":
		if !mihomo {
			return
		}
		o := host.Reality
		node.Set("tls", true)
		if node.Value("type") == "trojan" {
			node.Set("sni", o.ServerName)
		} else {
			node.Set("servername", o.ServerName)
		}
		if o.PublicKey != "" {
			opts := ojson.New("public-key", o.PublicKey, "short-id", o.ShortID)
			if host.MihomoX25519 {
				opts.Set("support-x25519mlkem768", true)
			}
			node.Set("reality-opts", opts)
		}
	}
}

func clashFingerprint(host *Resolved) string {
	raw := ""
	switch host.Security {
	case "tls":
		raw = strings.ToLower(host.TLS.Fingerprint)
	case "reality":
		raw = strings.ToLower(host.Reality.Fingerprint)
	}
	if raw == "" {
		return "chrome"
	}
	return resolveFingerprint(raw)
}

func clashWSOpts(rawPath *string, host string, headers any, httpUpgrade, mergeHeaders bool) *ojson.Object {
	config := ojson.New()
	path := ""
	if rawPath != nil {
		path = *rawPath
	}
	var maxEarlyData any = ojson.Undefined
	earlyName := ""
	if strings.Contains(path, "?ed=") {
		parts := strings.Split(path, "?ed=")
		path = parts[0]
		if parsed, ok := jsParseInt(strings.Split(parts[1], "/")[0]); ok && !math.IsNaN(parsed) {
			maxEarlyData = parsed
		}
		earlyName = "Sec-WebSocket-Protocol"
	}
	if path != "" {
		config.Set("path", path)
	}
	h := ojson.New()
	if host != "" {
		h.Set("Host", host)
	}
	if mergeHeaders && headers != nil {
		if o, ok := headers.(*ojson.Object); ok {
			h = ojson.New().Spread(h).Spread(o)
		}
	}
	config.Set("headers", h)
	if maxEarlyData != ojson.Undefined {
		config.Set("max-early-data", maxEarlyData)
	}
	if earlyName != "" {
		config.Set("early-data-header-name", earlyName)
	}
	if httpUpgrade {
		config.Set("v2ray-http-upgrade", true).Set("v2ray-http-upgrade-fast-open", true)
	}
	return config
}

func mihomoTransportOpts(node *ojson.Object, host *Resolved) {
	var opts *ojson.Object
	switch host.Transport {
	case "ws":
		opts = clashWSOpts(host.WS.Path, host.WS.Host, host.WS.Headers, false, true)
	case "httpupgrade":
		opts = clashWSOpts(host.HTTPUp.Path, host.HTTPUp.Host, host.HTTPUp.Headers, true, true)
	case "tcp":
		opts = ojson.New()
	case "grpc":
		opts = ojson.New("grpc-service-name", nullStrOrEmpty(host.GRPC.ServiceName))
	case "xhttp":
		opts = mihomoXhttpOpts(host.XHTTP, host.MihomoX25519)
	default:
		return
	}
	if opts.Len() > 0 {
		node.Set(jsString(node.Value("network"))+"-opts", opts)
	}
}

func nullStrOrEmpty(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func mihomoXhttpOpts(t *XHTTPOptions, x25519 bool) *ojson.Object {
	config := ojson.New()
	if t.Path != nil && *t.Path != "" {
		config.Set("path", *t.Path)
	}
	if t.Host != "" {
		config.Set("host", t.Host)
	}
	if ojson.Truthy(t.Mode) {
		config.Set("mode", t.Mode)
	}
	extra := t.Extra
	if extra == nil {
		return config
	}
	if ojson.Truthy(extra.Value("headers")) {
		config.Set("headers", extra.Value("headers"))
	}
	applyFieldMap(extra, config, xhttpFieldMap)
	if xmux, ok := extra.Value("xmux").(*ojson.Object); ok && xmux != nil {
		settings := ojson.New()
		applyFieldMap(xmux, settings, xmuxFieldMap)
		config.Set("reuse-settings", settings)
	}
	if ds, ok := extra.Value("downloadSettings").(*ojson.Object); ok && ds != nil {
		config.Set("download-settings", mihomoDownloadSettings(ds, x25519))
	}
	return config
}

func mihomoDownloadSettings(ds *ojson.Object, x25519 bool) *ojson.Object {
	settings := ojson.New()
	setTruthy(settings, "server", ds.Value("address"))
	setTruthy(settings, "port", ds.Value("port"))
	security := ds.Value("security")
	if security == "tls" || security == "reality" {
		settings.Set("tls", true)
		if tls, ok := ds.Value("tlsSettings").(*ojson.Object); ok && tls != nil {
			setTruthy(settings, "servername", tls.Value("serverName"))
			setTruthy(settings, "client-fingerprint", tls.Value("fingerprint"))
			setTruthy(settings, "alpn", tls.Value("alpn"))
			if ojson.Truthy(tls.Value("allowInsecure")) {
				settings.Set("skip-cert-verify", true)
			}
		}
		if reality, ok := ds.Value("realitySettings").(*ojson.Object); security == "reality" && ok && reality != nil {
			opts := ojson.New()
			setTruthy(opts, "public-key", reality.Value("publicKey"))
			setTruthy(opts, "short-id", reality.Value("shortId"))
			if x25519 {
				opts.Set("support-x25519mlkem768", true)
			}
			if opts.Len() > 0 {
				settings.Set("reality-opts", opts)
			}
		}
	}
	if xs, ok := ds.Value("xhttpSettings").(*ojson.Object); ok && xs != nil {
		setTruthy(settings, "path", xs.Value("path"))
		setTruthy(settings, "host", xs.Value("host"))
		setTruthy(settings, "headers", xs.Value("headers"))
		if xmux, ok := get(xs, "extra", "xmux").(*ojson.Object); ok && xmux != nil {
			reuse := ojson.New()
			applyFieldMap(xmux, reuse, xmuxFieldMap)
			settings.Set("reuse-settings", reuse)
		}
	}
	return settings
}

func mihomoHysteria2(host *Resolved, isExtendedClient bool) *ojson.Object {
	if host.Protocol != "hysteria" || host.Transport != "hysteria" {
		return nil
	}
	node := ojson.New("name", host.FinalRemark, "type", "hysteria2", "server", host.Address, "port", float64(host.Port), "udp", true, "password", host.Hysteria.Auth)
	quic := get(host.FinalMask, "quicParams")
	if v := get(quic, "brutalUp"); ojson.Truthy(v) {
		node.Set("up", jsString(v))
	}
	if v := get(quic, "brutalDown"); ojson.Truthy(v) {
		node.Set("down", jsString(v))
	}
	if v := get(quic, "udpHop", "ports"); ojson.Truthy(v) {
		node.Set("ports", jsString(v))
	}
	if v := get(quic, "udpHop", "interval"); ojson.Truthy(v) {
		node.Set("hop-interval", jsString(v))
	}
	if v := get(quic, "bbrProfile"); ojson.Truthy(v) {
		node.Set("bbr-profile", v)
	}
	if mask := findHysteria2Mask(host.FinalMask); mask != nil {
		password := get(mask, "settings", "password")
		packetSize, hasSize := getDefined(mask, "settings", "packetSize")
		if !hasSize || !ojson.Truthy(packetSize) {
			node.Set("obfs", "salamander").Set("obfs-password", password)
		} else {
			from, to := parseIntRange(packetSize)
			node.Set("obfs", "gecko").Set("obfs-password", password)
			if from != nil && *from != 0 {
				node.Set("obfs-min-packet-size", *from)
			}
			if to != nil && *to != 0 {
				node.Set("obfs-max-packet-size", *to)
			}
		}
	}
	if host.Security == "tls" {
		o := host.TLS
		if o.ServerName != "" {
			node.Set("sni", o.ServerName)
		}
		if o.PinnedPeerCertSha256 != nil && *o.PinnedPeerCertSha256 != "" {
			node.Set("skip-cert-verify", true)
		}
		if o.Fingerprint != "" {
			node.Set("client-fingerprint", o.Fingerprint)
		}
		if o.ALPN != "" {
			node.Set("alpn", splitAny(o.ALPN, ","))
		}
	}
	if isExtendedClient && host.ServerDescription != nil && *host.ServerDescription != "" {
		node.Set("serverDescription", js.DecodeBase64String(*host.ServerDescription))
	}
	return node
}

func findHysteria2Mask(finalMask *ojson.Object) any {
	udp, ok := get(finalMask, "udp").([]any)
	if !ok {
		return nil
	}
	for _, mask := range udp {
		object, ok := mask.(*ojson.Object)
		if !ok || object == nil || !object.Has("type") || !object.Has("settings") || object.Value("type") != "salamander" {
			continue
		}
		settings, ok := object.Value("settings").(*ojson.Object)
		if !ok || settings == nil || !settings.Has("password") {
			continue
		}
		if password, ok := settings.Value("password").(string); !ok || password == "" {
			continue
		}
		if size, has := settings.Get("packetSize"); has {
			switch size.(type) {
			case string, float64:
			default:
				if size != ojson.Undefined {
					continue
				}
			}
		}
		return object
	}
	return nil
}

// GenerateClash ports ClashGeneratorService.generateConfig.
func GenerateClash(hosts []*Resolved, template *ojson.Object) (string, error) {
	var proxies []any
	var remarks []string
	for _, host := range hosts {
		if contains(host.Meta.ExcludeFromSubscriptionTypes, "CLASH") || host.Transport == "hysteria" || host.Transport == "kcp" || host.Transport == "xhttp" || host.Protocol == "hysteria" || host.Protocol == "vless" {
			continue
		}
		node := ojson.New("name", host.FinalRemark, "type", clashType(host.Protocol), "server", host.Address, "port", float64(host.Port), "network", clashNetwork(host), "udp", true)
		switch host.Protocol {
		case "trojan":
			node.Set("password", host.Password)
		case "shadowsocks":
			node.Set("password", host.Password).Set("cipher", host.Method)
		default:
			continue
		}
		clashSecurityFields(node, host, false)
		var opts *ojson.Object
		switch host.Transport {
		case "ws":
			opts = clashWSOpts(host.WS.Path, host.WS.Host, nil, false, false)
		case "httpupgrade":
			opts = clashWSOpts(host.HTTPUp.Path, host.HTTPUp.Host, nil, true, false)
		case "grpc":
			opts = ojson.New("grpc-service-name", nullStrOrEmpty(host.GRPC.ServiceName))
		default:
			opts = ojson.New()
		}
		if opts.Len() > 0 {
			node.Set(jsString(node.Value("network"))+"-opts", opts)
		}
		node.Set("client-fingerprint", clashFingerprint(host))
		proxies = append(proxies, node)
		remarks = append(remarks, host.FinalRemark)
	}
	return renderClashFamily(template, proxies, remarks, false)
}

// renderClashFamily merges proxies and proxy-group remarks into the template;
// mihomo also strips "remnawave" and fills proxy-providers.
func renderClashFamily(template *ojson.Object, proxies []any, remarks []string, mihomo bool) (string, error) {
	templateConfig := ojson.New()
	for _, key := range template.Keys() {
		if mihomo && key == "remnawave" {
			continue
		}
		templateConfig.Set(key, template.Value(key))
	}
	allProxies := []any{}
	if existing, ok := template.Value("proxies").([]any); ok {
		allProxies = append(allProxies, existing...)
	}
	allProxies = append(allProxies, proxies...)
	sourceGroups, _ := templateConfig.Value("proxy-groups").([]any)
	groups := []any{}
	for _, raw := range sourceGroups {
		group, _ := raw.(*ojson.Object)
		custom, _ := get(group, "remnawave").(*ojson.Object)
		clean := group
		if custom != nil {
			clean = ojson.New()
			for _, key := range group.Keys() {
				if key != "remnawave" {
					clean.Set(key, group.Value(key))
				}
			}
		}
		names := []any{}
		if existing, ok := get(clean, "proxies").([]any); ok {
			names = append(names, existing...)
		}
		for _, remark := range groupRemarks(custom, remarks, mihomo) {
			names = append(names, remark)
		}
		groups = append(groups, ojson.New().Spread(clean).Set("proxies", names))
	}
	final := ojson.New().Spread(templateConfig).Set("proxies", allProxies).Set("proxy-groups", groups)
	if mihomo {
		if providers, ok := template.Value("proxy-providers").(*ojson.Object); ok && providers != nil {
			out := ojson.New()
			for _, key := range providers.Keys() {
				provider, _ := providers.Value(key).(*ojson.Object)
				custom, _ := get(provider, "remnawave").(*ojson.Object)
				if custom == nil {
					out.Set(key, providers.Value(key))
					continue
				}
				clean := ojson.New()
				for _, k := range provider.Keys() {
					if k != "remnawave" {
						clean.Set(k, provider.Value(k))
					}
				}
				if custom.Value("include-proxies") == true {
					clean = ojson.New().Spread(clean).Set("payload", append([]any{}, proxies...))
				}
				out.Set(key, clean)
			}
			final.Set("proxy-providers", out)
		}
	}
	return DumpYAML(final)
}

func groupRemarks(custom *ojson.Object, remarks []string, mihomo bool) []string {
	copyOf := func() []string { return append([]string{}, remarks...) }
	if custom == nil {
		return copyOf()
	}
	if custom.Value("include-proxies") == false {
		return []string{}
	}
	if custom.Value("select-random-proxy") == true {
		if len(remarks) == 0 {
			return []string{}
		}
		return []string{remarks[Random(len(remarks))]}
	}
	if custom.Value("shuffle-proxies-order") == true {
		list := copyOf()
		shuffle(list)
		return list
	}
	return copyOf()
}
