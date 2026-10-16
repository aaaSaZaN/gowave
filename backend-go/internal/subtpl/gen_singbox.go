package subtpl

import (
	"math"
	"regexp"
	"strconv"
	"strings"

	"gowave/internal/subtpl/ojson"
)

var (
	singboxProxyTypes    = map[string]bool{"hysteria2": true, "shadowsocks": true, "trojan": true, "vless": true}
	singboxSelectorTypes = map[string]bool{"hysteria2": true, "shadowsocks": true, "trojan": true, "vless": true, "urltest": true}
	multiplexProtocols   = map[string]bool{"h2mux": true, "smux": true, "yamux": true}
	durationRegex        = regexp.MustCompile(`^\d+(\.\d+)?(ns|us|µs|ms|s|m|h)$`)
	fingerprints         = []string{"randomized", "android", "firefox", "chrome", "safari", "edge", "ios", "qq", "360", "random"}
)

// GenerateSingbox ports SingBoxGeneratorService.generateConfig.
func GenerateSingbox(hosts []*Resolved, template *ojson.Object) string {
	var outbounds []any
	for _, host := range hosts {
		if contains(host.Meta.ExcludeFromSubscriptionTypes, "SINGBOX") || host.Transport == "kcp" || host.Transport == "xhttp" {
			continue
		}
		if outbound := singboxOutbound(host); outbound != nil {
			outbounds = append(outbounds, outbound)
		}
	}
	return renderSingbox(template, outbounds)
}

func singboxOutbound(host *Resolved) any {
	var config *ojson.Object
	if host.Protocol == "hysteria" {
		config = singboxHysteria2(host)
	} else {
		config = ojson.New("type", host.Protocol, "tag", host.FinalRemark, "server", host.Address, "server_port", float64(host.Port))
		if !singboxProtocolFields(config, host) {
			return nil
		}
		singboxTransport(config, host)
		singboxSecurity(config, host)
		if udp, _ := config.Value("udp_over_tcp").(*ojson.Object); !ojson.Truthy(get(udp, "enabled")) {
			if multiplex := singboxMultiplex(host.Mux); multiplex != nil {
				config.Set("multiplex", multiplex)
			}
		}
	}
	if config == nil {
		return nil
	}
	return applyHostMapper(config, host.mapperOps("singbox"), host, false)
}

func singboxProtocolFields(config *ojson.Object, host *Resolved) bool {
	switch host.Protocol {
	case "vless":
		if host.Encryption != "" && host.Encryption != "none" {
			return false
		}
		config.Set("uuid", host.ID)
		if host.Flow == "xtls-rprx-vision" && host.Transport == "tcp" && host.Security != "none" {
			config.Set("flow", host.Flow)
		}
		return true
	case "trojan":
		config.Set("password", host.Password)
		return true
	case "shadowsocks":
		config.Set("password", host.Password).Set("method", host.Method)
		if host.UoT {
			uot := ojson.New("enabled", true)
			if host.UoTVersion == 1 {
				uot.Set("version", 1.0)
			}
			config.Set("network", "tcp").Set("udp_over_tcp", uot)
		}
		return true
	}
	return false
}

// parsePositiveInt ports parseInt(String(v).trim(), 10) > 0.
func parsePositiveInt(value any) (float64, bool) {
	var text string
	switch v := value.(type) {
	case float64:
		text = ojson.FormatNumber(v)
	case string:
		text = v
	default:
		return 0, false
	}
	parsed, ok := jsParseInt(strings.TrimSpace(text))
	if !ok || parsed <= 0 {
		return 0, false
	}
	return parsed, true
}

// jsParseInt is parseInt(s, 10): optional sign and leading digits.
func jsParseInt(s string) (float64, bool) {
	s = strings.TrimLeft(s, " \t\n\r\v\f")
	end := 0
	if end < len(s) && (s[end] == '+' || s[end] == '-') {
		end++
	}
	digits := end
	for end < len(s) && s[end] >= '0' && s[end] <= '9' {
		end++
	}
	if end == digits {
		return 0, false
	}
	value, err := strconv.ParseFloat(s[:end], 64)
	return value, err == nil
}

func singboxMultiplex(mux *ojson.Object) *ojson.Object {
	smux, _ := get(mux, "smux").(*ojson.Object)
	if !ojson.IsNonEmptyObject(smux) || smux.Value("enabled") != true {
		return nil
	}
	config := ojson.New("enabled", true)
	if protocol, ok := smux.Value("protocol").(string); ok && multiplexProtocols[protocol] {
		config.Set("protocol", protocol)
	}
	if v, ok := parsePositiveInt(smux.Value("max-connections")); ok {
		config.Set("max_connections", v)
	}
	if v, ok := parsePositiveInt(smux.Value("min-streams")); ok {
		config.Set("min_streams", v)
	}
	if v, ok := parsePositiveInt(smux.Value("max-streams")); ok {
		config.Set("max_streams", v)
	}
	if smux.Value("padding") == true {
		config.Set("padding", true)
	}
	if brutal, _ := smux.Value("brutal-opts").(*ojson.Object); ojson.IsNonEmptyObject(brutal) && brutal.Value("enabled") == true {
		up, okUp := parsePositiveInt(brutal.Value("up"))
		down, okDown := parsePositiveInt(brutal.Value("down"))
		if okUp && okDown {
			config.Set("brutal", ojson.New("enabled", true, "up_mbps", up, "down_mbps", down))
		}
	}
	return config
}

func singboxHysteria2(host *Resolved) *ojson.Object {
	if host.Transport != "hysteria" {
		return nil
	}
	config := ojson.New("type", "hysteria2", "tag", host.FinalRemark, "server", host.Address, "server_port", float64(host.Port), "password", host.Hysteria.Auth, "tls", singboxQuicTLS(host))
	quic := get(host.FinalMask, "quicParams")
	if up, ok := parsePositiveInt(get(quic, "brutalUp")); ok {
		config.Set("up_mbps", up)
	}
	if down, ok := parsePositiveInt(get(quic, "brutalDown")); ok {
		config.Set("down_mbps", down)
	}
	if ports := parsePortRanges(get(quic, "udpHop", "ports")); len(ports) > 0 {
		config.Set("server_ports", ports)
		if interval, ok := parseDuration(get(quic, "udpHop", "interval")); ok {
			config.Set("hop_interval", interval)
		}
	}
	if udp, ok := get(host.FinalMask, "udp").([]any); ok {
		for _, mask := range udp {
			if get(mask, "type") == "salamander" && ojson.Truthy(get(mask, "settings", "password")) {
				config.Set("obfs", ojson.New("type", "salamander", "password", get(mask, "settings", "password")))
				break
			}
		}
	}
	return config
}

// parseIntRange ports parseIntRangeUtil.
func parseIntRange(value any) (from, to *float64) {
	if value == nil || value == "" {
		return nil, nil
	}
	text := jsString(value)
	left, right, found := strings.Cut(text, "-")
	if !found {
		right = left
	} else if i := strings.Index(right, "-"); i >= 0 {
		right = right[:i]
	}
	part := func(p string) *float64 {
		if p == "" {
			return nil
		}
		f := jsNumber(p)
		if math.IsNaN(f) || f != math.Trunc(f) || f < 0 || math.IsInf(f, 0) {
			return nil
		}
		return &f
	}
	from, to = part(left), part(right)
	if from != nil && to != nil && *from > *to {
		return to, from
	}
	return from, to
}

func parsePortRanges(value any) []any {
	if value == nil || value == "" {
		return nil
	}
	var ranges []any
	for _, part := range strings.Split(jsString(value), ",") {
		from, to := parseIntRange(strings.TrimSpace(part))
		if from == nil || *from == 0 || *from > 65535 {
			continue
		}
		end := *from
		if to != nil && *to <= 65535 {
			end = *to
		}
		ranges = append(ranges, ojson.FormatNumber(*from)+":"+ojson.FormatNumber(end))
	}
	return ranges
}

func parseDuration(value any) (string, bool) {
	if value == nil || value == "" {
		return "", false
	}
	raw := strings.TrimSpace(jsString(value))
	if regexp.MustCompile(`^\d+$`).MatchString(raw) {
		if jsNumber(raw) > 0 {
			return raw + "s", true
		}
		return "", false
	}
	return raw, durationRegex.MatchString(raw)
}

func singboxTransport(config *ojson.Object, host *Resolved) {
	switch host.Transport {
	case "ws":
		config.Set("transport", singboxWS(host.WS.Path, host.WS.Host, host.WS.Headers))
	case "httpupgrade":
		t := ojson.New("type", "httpupgrade")
		if host.HTTPUp.Path != nil && *host.HTTPUp.Path != "" {
			t.Set("path", *host.HTTPUp.Path)
		}
		if host.HTTPUp.Host != "" {
			t.Set("host", host.HTTPUp.Host)
		}
		if headers := singboxHeaders(host.HTTPUp.Headers, ""); headers != nil {
			t.Set("headers", headers)
		}
		config.Set("transport", t)
	case "grpc":
		name := ""
		if host.GRPC.ServiceName != nil {
			name = *host.GRPC.ServiceName
		}
		config.Set("transport", ojson.New("type", "grpc", "service_name", name))
	}
}

func singboxWS(rawPath *string, host string, headers any) *ojson.Object {
	config := ojson.New("type", "ws")
	path := ""
	if rawPath != nil {
		path = *rawPath
	}
	if strings.Contains(path, "?ed=") {
		parts := strings.Split(path, "?ed=")
		path = parts[0]
		if parsed := jsNumber(strings.Split(parts[1], "/")[0]); !math.IsNaN(parsed) {
			config.Set("max_early_data", parsed)
		}
		config.Set("early_data_header_name", "Sec-WebSocket-Protocol")
	}
	if path != "" {
		config.Set("path", path)
	}
	if h := singboxHeaders(headers, host); h != nil {
		config.Set("headers", h)
	}
	return config
}

func singboxHeaders(raw any, host string) *ojson.Object {
	headers := ojson.New()
	if o, ok := raw.(*ojson.Object); ok && o != nil {
		for _, key := range o.Keys() {
			if strings.ToLower(key) == "host" && host == "" {
				continue
			}
			if value, ok := o.Value(key).(string); ok {
				headers.Set(key, value)
			}
		}
	}
	if host != "" {
		headers.Set("Host", host)
	}
	if headers.Len() == 0 {
		return nil
	}
	return headers
}

func resolveFingerprint(fingerprint string) string {
	for _, fp := range fingerprints {
		if strings.Contains(fingerprint, fp) {
			return fp
		}
	}
	return "chrome"
}

func trimmedList(alpn string) []any {
	parts := splitAny(alpn, ",")
	for i, part := range parts {
		parts[i] = strings.TrimSpace(part.(string))
	}
	return parts
}

func singboxSecurity(config *ojson.Object, host *Resolved) {
	switch host.Security {
	case "tls":
		o := host.TLS
		tls := ojson.New("enabled", true)
		if o.ServerName != "" {
			tls.Set("server_name", o.ServerName)
		}
		if o.Fingerprint != "" {
			tls.Set("utls", ojson.New("enabled", true, "fingerprint", resolveFingerprint(o.Fingerprint)))
		}
		if o.PinnedPeerCertSha256 != nil && *o.PinnedPeerCertSha256 != "" {
			tls.Set("insecure", true)
		}
		if o.ALPN != "" {
			tls.Set("alpn", trimmedList(o.ALPN))
		}
		config.Set("tls", tls)
	case "reality":
		o := host.Reality
		reality := ojson.New("enabled", true)
		tls := ojson.New("enabled", true, "reality", reality)
		if o.ServerName != "" {
			tls.Set("server_name", o.ServerName)
		}
		if o.PublicKey != "" {
			reality.Set("public_key", o.PublicKey)
		}
		if ojson.Truthy(o.ShortID) {
			reality.Set("short_id", o.ShortID)
		}
		tls.Set("utls", ojson.New("enabled", true, "fingerprint", resolveFingerprint(o.Fingerprint)))
		config.Set("tls", tls)
	}
}

func singboxQuicTLS(host *Resolved) *ojson.Object {
	tls := ojson.New("enabled", true)
	if host.Security != "tls" {
		return tls
	}
	o := host.TLS
	if o.ServerName != "" {
		tls.Set("server_name", o.ServerName)
	}
	if o.PinnedPeerCertSha256 != nil && *o.PinnedPeerCertSha256 != "" {
		tls.Set("insecure", true)
	}
	if o.ALPN != "" {
		tls.Set("alpn", trimmedList(o.ALPN))
	}
	return tls
}

func renderSingbox(template *ojson.Object, userOutbounds []any) string {
	templateOutbounds, _ := template.Value("outbounds").([]any)
	all := append(append([]any{}, templateOutbounds...), userOutbounds...)
	var urltest, selector []any
	for _, outbound := range all {
		kind := asString(get(outbound, "type"))
		tag := get(outbound, "tag")
		if tag == nil {
			tag = ojson.Undefined
		}
		if singboxProxyTypes[kind] {
			urltest = append(urltest, tag)
		}
		if singboxSelectorTypes[kind] {
			selector = append(selector, tag)
		}
	}
	if urltest == nil {
		urltest = []any{}
	}
	if selector == nil {
		selector = []any{}
	}
	final := make([]any, 0, len(all))
	for _, raw := range all {
		outbound, ok := raw.(*ojson.Object)
		if !ok {
			final = append(final, raw)
			continue
		}
		clean := ojson.New()
		for _, key := range outbound.Keys() {
			if key != "remnawave" {
				clean.Set(key, outbound.Value(key))
			}
		}
		if get(outbound, "remnawave", "includeProxies") == false {
			final = append(final, clean)
			continue
		}
		switch clean.Value("type") {
		case "urltest":
			clean = ojson.New().Spread(clean).Set("outbounds", urltest)
		case "selector":
			clean = ojson.New().Spread(clean).Set("outbounds", selector)
		}
		final = append(final, clean)
	}
	return ojson.Stringify(ojson.New().Spread(template).Set("outbounds", final))
}
