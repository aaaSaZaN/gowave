package subtpl

import (
	"crypto/rand"
	"math/big"
	"regexp"
	"strings"

	"remnawave-go/internal/subtpl/js"
	"remnawave-go/internal/subtpl/ojson"
)

// ResolveOptions mirrors IResolveProxyConfigOptions.
type ResolveOptions struct {
	Settings           *Settings
	Hosts              []*Host
	User               *User
	HostOverrides      *HostOverrides
	Fallback           *FallbackOptions
	ExcludeHostsByTags map[string]bool
	SubPublicDomain    string
}

var domainRegex = regexp.MustCompile(`^(?:[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$`)

// Random is the randomness source for shuffles and "a,b" / "*" values; the
// diff harness pins it.
var Random = func(n int) int {
	value, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if err != nil {
		return 0
	}
	return int(value.Int64())
}

const randomAlphabet = "0123456789abcdefghjkmnopqrstuvwxyz"

// resolveRandomizedValue picks one of "a,b,c" or fills the first "*".
func resolveRandomizedValue(value string) string {
	if value == "" {
		return value
	}
	if strings.Contains(value, ",") {
		parts := strings.Split(value, ",")
		return strings.TrimSpace(parts[Random(len(parts))])
	}
	if strings.Contains(value, "*") {
		id := make([]byte, 10)
		for i := range id {
			id[i] = randomAlphabet[Random(len(randomAlphabet))]
		}
		return strings.TrimSpace(strings.Replace(value, "*", string(id), 1))
	}
	return value
}

// override ports override(): (host || inbound) ?? null with JS truthiness.
func override(hostValue, inboundValue any) any {
	if ojson.Truthy(hostValue) {
		return hostValue
	}
	if inboundValue == ojson.Undefined {
		return nil
	}
	return inboundValue
}

func ptrAny(p *string) any {
	if p == nil {
		return nil
	}
	return *p
}

func asStringPtr(v any) *string {
	if s, ok := v.(string); ok {
		return &s
	}
	return nil
}

func asString(v any) string {
	s, _ := v.(string)
	return s
}

// toNonEmptyRecord ports toNonEmptyRecord: any non-null value with at least
// one key (objects, non-empty arrays/strings index keys too).
func toNonEmptyRecord(value any) *ojson.Object {
	if o, ok := value.(*ojson.Object); ok && o != nil && o.Len() > 0 {
		return o
	}
	return nil
}

func get(root any, path ...string) any {
	value, ok := ojson.Get(root, path)
	if !ok {
		return nil
	}
	return value
}

func getDefined(root any, path ...string) (any, bool) {
	return ojson.Get(root, path)
}

// ResolveProxyConfig ports ResolveProxyConfigService.resolveProxyConfig.
func ResolveProxyConfig(options ResolveOptions) []*Resolved {
	if options.Settings == nil {
		return nil
	}
	hosts := options.Hosts
	if len(options.ExcludeHostsByTags) > 0 {
		var kept []*Host
		for _, host := range hosts {
			excluded := false
			for _, tag := range host.Tags {
				if options.ExcludeHostsByTags[tag] {
					excluded = true
					break
				}
			}
			if !excluded {
				kept = append(kept, host)
			}
		}
		hosts = kept
	}
	values := TemplateValues(options.User, options.Settings, options.SubPublicDomain)
	if early := earlyExitRemarks(options.User, options.Settings, options.Fallback, len(hosts)); early != nil {
		remarks := make([]string, len(early))
		for i, remark := range early {
			remarks[i] = ReplaceTemplate(remark, values)
		}
		return createFallbackHosts(remarks)
	}
	hosts = applyShuffle(hosts)

	rawInbounds := make([]any, len(hosts))
	for i, host := range hosts {
		rawInbounds[i] = host.RawInbound
	}
	publicKeys, mldsa65Keys, encryption := inboundKeys(rawInbounds)

	known := map[string]int{}
	var resolved []*Resolved
	for _, input := range hosts {
		host := *input
		applyHostOverrides(&host, options.HostOverrides)
		remark := deduplicateRemark(ReplaceTemplate(host.Remark, values), known)
		if config := buildResolved(&host, remark, options.User, publicKeys, mldsa65Keys, encryption); config != nil {
			resolved = append(resolved, config)
		}
	}
	return resolved
}

func earlyExitRemarks(user *User, settings *Settings, fallback *FallbackOptions, hostCount int) []string {
	if settings.IsShowCustomRemarks {
		if fallback != nil {
			if fallback.ShowHwidMaxDeviceRemarks {
				return nonNil(settings.CustomRemarks.HWIDMaxDevicesExceeded)
			}
			if fallback.ShowHwidNotSupportedRemarks {
				return nonNil(settings.CustomRemarks.HWIDNotSupported)
			}
			if len(fallback.RespondWithRemarks) > 0 {
				return fallback.RespondWithRemarks
			}
		}
		if user.Status != "ACTIVE" {
			switch user.Status {
			case "EXPIRED":
				return nonNil(settings.CustomRemarks.ExpiredUsers)
			case "DISABLED":
				return nonNil(settings.CustomRemarks.DisabledUsers)
			case "LIMITED":
				return nonNil(settings.CustomRemarks.LimitedUsers)
			}
			return []string{}
		}
	}
	if hostCount == 0 {
		return nonNil(settings.CustomRemarks.EmptyHosts)
	}
	return nil
}

func nonNil(list []string) []string {
	if list == nil {
		return []string{}
	}
	return list
}

func applyShuffle(hosts []*Host) []*Host {
	var shuffled, rest []*Host
	for _, host := range hosts {
		if host.ShuffleHost {
			shuffled = append(shuffled, host)
		} else {
			rest = append(rest, host)
		}
	}
	if len(shuffled) == 0 {
		return hosts
	}
	shuffle(shuffled)
	return append(shuffled, rest...)
}

// shuffle is lodash's Fisher–Yates variant.
func shuffle[T any](list []T) {
	for index := 0; index < len(list)-1; index++ {
		rand := index + Random(len(list)-index)
		list[index], list[rand] = list[rand], list[index]
	}
}

func applyHostOverrides(host *Host, overrides *HostOverrides) {
	if overrides == nil {
		return
	}
	if overrides.VlessRouteID != nil {
		host.VlessRouteID = *overrides.VlessRouteID
	}
	if overrides.ServerDescription != nil {
		host.ServerDescription = *overrides.ServerDescription
	}
}

func deduplicateRemark(remark string, known map[string]int) string {
	count := known[remark]
	known[remark] = count + 1
	if count == 0 {
		return remark
	}
	suffix := count + 1
	if strings.Contains(remark, "^~") && strings.HasSuffix(remark, "~^") {
		suffix = count
	}
	return remark + " ^~" + itoa(suffix) + "~^"
}

func itoa(n int) string { return ojson.FormatNumber(float64(n)) }

const hexChars = "0123456789abcdef"

// setVlessRouteForUuid embeds a 16-bit route id into the UUID's third group.
func setVlessRouteForUuid(uuid string, routeID *int) string {
	if routeID == nil || *routeID == 0 || len(uuid) < 18 {
		return uuid
	}
	id := *routeID
	return uuid[:14] + string([]byte{hexChars[(id>>12)&0xf], hexChars[(id>>8)&0xf], hexChars[(id>>4)&0xf], hexChars[id&0xf]}) + uuid[18:]
}

func getVlessFlow(inbound any) string {
	if asString(get(inbound, "protocol")) != "vless" {
		return ""
	}
	settings, ok := get(inbound, "settings").(*ojson.Object)
	if !ok || settings == nil {
		return ""
	}
	if flow, ok := settings.Get("flow"); ok && flow != ojson.Undefined {
		if flow == "xtls-rprx-vision" {
			return "xtls-rprx-vision"
		}
		return ""
	}
	if stream, ok := get(inbound, "streamSettings").(*ojson.Object); ok && stream != nil {
		security := asString(stream.Value("security"))
		network := asString(stream.Value("network"))
		if (security == "reality" || security == "tls") && (network == "raw" || network == "tcp") {
			return "xtls-rprx-vision"
		}
	}
	return ""
}

func resolveProtocol(host *Host, inbound any, user *User, encryption string, hasEncryption bool) (*Resolved, bool) {
	if !ojson.Truthy(get(inbound, "settings")) {
		return nil, false
	}
	r := &Resolved{}
	switch asString(get(inbound, "protocol")) {
	case "vless":
		r.Protocol = "vless"
		r.ID = setVlessRouteForUuid(user.VlessUUID, host.VlessRouteID)
		r.Encryption = "none"
		if hasEncryption {
			r.Encryption = encryption
		}
		r.Flow = getVlessFlow(inbound)
	case "trojan":
		r.Protocol = "trojan"
		r.Password = user.TrojanPassword
	case "shadowsocks":
		r.Protocol = "shadowsocks"
		method := asString(get(inbound, "settings", "method"))
		password := user.SsPassword
		if serverPassword, ok := getDefined(inbound, "settings", "password"); ok && method == "2022-blake3-aes-256-gcm" {
			password = jsString(serverPassword) + ":" + js.Base64(user.SsPassword)
		}
		if method == "" {
			method = "chacha20-ietf-poly1305"
		}
		r.Method = method
		r.Password = password
		r.UoT = ojson.Truthy(get(inbound, "settings", "uot"))
		r.UoTVersion = 1
		if v, ok := get(inbound, "settings", "uotVersion").(float64); ok && v != 0 {
			r.UoTVersion = v
		}
	case "hysteria":
		r.Protocol = "hysteria"
		r.Version = 2
	default:
		return nil, false
	}
	return r, true
}

// jsString is String(value) for JSON values.
func jsString(value any) string {
	switch v := value.(type) {
	case nil:
		return "null"
	case string:
		return v
	case bool:
		if v {
			return "true"
		}
		return "false"
	case float64:
		return ojson.FormatNumber(v)
	case *ojson.Object:
		return "[object Object]"
	case []any:
		parts := make([]string, len(v))
		for i, item := range v {
			if item != nil && item != ojson.Undefined {
				parts[i] = jsString(item)
			}
		}
		return strings.Join(parts, ",")
	}
	return ""
}

func resolveTransport(r *Resolved, stream any, host *Host, vlessUUID string) {
	network, defined := getDefined(stream, "network")
	streamObject, _ := stream.(*ojson.Object)
	if !defined || streamObject == nil {
		r.Transport, r.TCP = "tcp", &TCPOptions{}
		return
	}
	hostPath, hostHost := ptrAny(host.Path), ptrAny(host.Host)
	randomized := func(value any) string {
		s, _ := value.(string)
		return resolveRandomizedValue(s)
	}
	switch network {
	case "xhttp":
		settings := get(stream, "xhttpSettings")
		mode, ok := getDefined(settings, "mode")
		if !ok || mode == nil {
			mode = "auto"
		}
		extra := override(objOrNull(toNonEmptyRecord(host.XhttpExtraParams)), get(settings, "extra"))
		extraObject, _ := extra.(*ojson.Object)
		r.Transport = "xhttp"
		r.XHTTP = &XHTTPOptions{Path: asStringPtr(override(hostPath, get(settings, "path"))), Host: randomized(nullishString(override(hostHost, get(settings, "host")))), Mode: mode, Extra: extraObject}
	case "ws":
		settings := get(stream, "wsSettings")
		r.Transport = "ws"
		r.WS = &WSOptions{Host: randomized(nullishString(override(hostHost, get(settings, "host")))), Path: asStringPtr(override(hostPath, get(settings, "path"))), Headers: nullish(get(settings, "headers")), HeartbeatPeriod: nullish(get(settings, "heartbeatPeriod"))}
	case "httpupgrade":
		settings := get(stream, "httpupgradeSettings")
		r.Transport = "httpupgrade"
		r.HTTPUp = &HTTPUpgradeOptions{Path: asStringPtr(override(hostPath, get(settings, "path"))), Host: randomized(nullishString(override(hostHost, get(settings, "host")))), Headers: nullish(get(settings, "headers"))}
	case "grpc":
		settings := get(stream, "grpcSettings")
		r.Transport = "grpc"
		r.GRPC = &GRPCOptions{Authority: randomized(nullishString(override(hostHost, get(settings, "authority")))), ServiceName: asStringPtr(override(hostPath, get(settings, "serviceName"))), MultiMode: ojson.Truthy(get(settings, "multiMode"))}
	case "raw":
		resolveTCP(r, get(stream, "rawSettings"), host)
	case "tcp":
		resolveTCP(r, get(stream, "tcpSettings"), host)
	case "kcp":
		settings := get(stream, "kcpSettings")
		r.Transport = "kcp"
		r.KCP = &KCPOptions{
			ClientMtu:  firstTruthy(get(settings, "clientMtu"), get(settings, "mtu"), 1350.0),
			ClientTti:  firstTruthy(get(settings, "clientTti"), get(settings, "tti"), 50.0),
			Congestion: firstTruthy(get(settings, "congestion"), false),
		}
	case "hysteria":
		auth := ""
		if r.Protocol == "hysteria" {
			auth = setVlessRouteForUuid(vlessUUID, host.VlessRouteID)
		} else if a := get(stream, "hysteriaSettings", "auth"); ojson.Truthy(a) {
			auth = asString(a)
		}
		r.Transport = "hysteria"
		r.Hysteria = &HysteriaOptions{Version: 2, Auth: auth}
	default:
		r.Transport, r.TCP = "tcp", &TCPOptions{}
	}
}

func nullish(value any) any {
	if value == ojson.Undefined {
		return nil
	}
	return value
}

func nullishString(value any) any {
	if value == nil {
		return ""
	}
	return value
}

func firstTruthy(values ...any) any {
	for _, value := range values[:len(values)-1] {
		if ojson.Truthy(value) {
			return value
		}
	}
	return values[len(values)-1]
}

func resolveTCP(r *Resolved, settings any, host *Host) {
	r.Transport = "tcp"
	header, _ := get(settings, "header").(*ojson.Object)
	if header != nil && header.Value("type") == "http" {
		request, _ := ojson.Clone(header.Value("request")).(*ojson.Object)
		if request == nil {
			request = ojson.New("version", "1.1", "method", "GET", "headers", ojson.New(
				"Accept-Encoding", []any{"gzip", "deflate"},
				"Connection", []any{"keep-alive"},
				"Pragma", []any{"no-cache"},
			))
		} else {
			headers, _ := request.Value("headers").(*ojson.Object)
			if !ojson.Truthy(request.Value("headers")) {
				headers = ojson.New()
				request.Set("headers", headers)
			}
			if host.Host != nil && *host.Host != "" && headers != nil {
				headers.Set("Host", []any{resolveRandomizedValue(*host.Host)})
			}
			if host.Path != nil && *host.Path != "" {
				request.Set("path", []any{*host.Path})
			}
		}
		r.TCP = &TCPOptions{Header: ojson.New("type", "http", "request", request)}
		return
	}
	headerValue := get(settings, "header")
	r.TCP = &TCPOptions{Header: nullish(headerValue)}
}

func resolveFinalServerName(host *Host, serverName any, resolvedAddress string) string {
	if host.KeepSniBlank {
		return ""
	}
	if host.OverrideSniFromAddress {
		return resolvedAddress
	}
	base := ""
	if serverName != nil {
		base = jsString(serverName)
	}
	if host.SNI != nil && *host.SNI != "" {
		base = *host.SNI
	}
	if base == "" && domainRegex.MatchString(host.Address) {
		base = host.Address
	}
	return resolveRandomizedValue(base)
}

func resolveSecurity(r *Resolved, stream any, host *Host, inboundTag string, publicKeys, mldsa65Keys map[string]string, address string) {
	streamObject, _ := stream.(*ojson.Object)
	if streamObject == nil {
		r.Security = "none"
		return
	}
	security := streamObject.Value("security")
	switch host.SecurityLayer {
	case "TLS":
		security = "tls"
	case "NONE":
		security = "none"
	}
	switch security {
	case "tls":
		tls := get(stream, "tlsSettings")
		alpnValue := get(tls, "alpn")
		if list, ok := alpnValue.([]any); ok {
			alpnValue = jsString(list)
		}
		alpn := nullishString(override(ptrAny(host.ALPN), alpnValue))
		fingerprint := override(ptrAny(host.Fingerprint), get(tls, "fingerprint"))
		if fingerprint == nil {
			fingerprint = "chrome"
		}
		r.Security = "tls"
		r.TLS = &TLSOptions{
			ALPN:                    jsString(alpn),
			EnableSessionResumption: ojson.Truthy(get(tls, "enableSessionResumption")),
			Fingerprint:             jsString(fingerprint),
			ServerName:              resolveFinalServerName(host, rawServerName(get(stream, "tlsSettings", "serverName")), address),
			EchConfigList:           orNull(get(tls, "echConfigList")),
			EchForceQuery:           orNull(get(tls, "echForceQuery")),
			EchSockopt:              toNonEmptyRecord(get(tls, "echSockopt")),
			PinnedPeerCertSha256:    host.PinnedPeerCertSha256,
			VerifyPeerCertByName:    host.VerifyPeerCertByName,
			CipherSuites:            orNull(get(tls, "cipherSuites")),
		}
	case "reality":
		reality := get(stream, "realitySettings")
		var shortID any = ""
		if ids, ok := get(reality, "shortIds").([]any); ok && len(ids) > 0 {
			shortID = nullish(ids[0])
		}
		fingerprint := override(ptrAny(host.Fingerprint), get(reality, "fingerprint"))
		if fingerprint == nil {
			fingerprint = "chrome"
		}
		var mldsa *string
		if value, ok := mldsa65Keys[inboundTag]; ok {
			mldsa = &value
		}
		r.Security = "reality"
		r.Reality = &RealityOptions{
			Fingerprint:   jsString(fingerprint),
			PublicKey:     publicKeys[inboundTag],
			ShortID:       shortID,
			ServerName:    resolveFinalServerName(host, rawServerName(get(reality, "serverNames", "0")), address),
			SpiderX:       firstTruthy(get(reality, "spiderX"), ""),
			Mldsa65Verify: mldsa,
		}
	default:
		r.Security = "none"
	}
}

func rawServerName(value any) any {
	if value == ojson.Undefined {
		return nil
	}
	return value
}

func orNull(value any) any {
	if ojson.Truthy(value) {
		return value
	}
	return nil
}

func buildResolved(host *Host, remark string, user *User, publicKeys, mldsa65Keys, encryption map[string]string) *Resolved {
	inbound := host.RawInbound
	address := resolveRandomizedValue(host.Address)
	enc, hasEnc := encryption[host.InboundTag]
	r, ok := resolveProtocol(host, inbound, user, enc, hasEnc)
	if !ok {
		return nil
	}
	stream := get(inbound, "streamSettings")
	resolveTransport(r, stream, host, user.VlessUUID)
	inboundTag := asString(get(inbound, "tag"))
	resolveSecurity(r, stream, host, inboundTag, publicKeys, mldsa65Keys, address)

	r.FinalRemark = remark
	r.Address = address
	r.Port = host.Port
	r.FinalMask, _ = override(objOrNull(toNonEmptyRecord(host.FinalMask)), objOrNull(toNonEmptyRecord(get(stream, "finalmask")))).(*ojson.Object)
	r.Sockopt = toNonEmptyRecord(host.SockoptParams)
	r.Mux = toNonEmptyRecord(host.MuxParams)
	r.ShuffleHost = host.ShuffleHost
	r.MihomoX25519 = host.MihomoX25519
	r.MihomoIPVersion = host.MihomoIPVersion
	if host.ServerDescription != nil && *host.ServerDescription != "" {
		encoded := js.Base64(*host.ServerDescription)
		r.ServerDescription = &encoded
	}
	r.XrayJSONTemplate = host.XrayJSONTemplate
	r.Mapper = host.Mapper
	if r.Mapper == nil {
		r.Mapper = ojson.New()
	}
	r.Meta = Metadata{
		UUID: host.UUID, Tags: nonNil(host.Tags), ExcludeFromSubscriptionTypes: nonNil(host.ExcludeFromSubscriptionTypes),
		InboundTag: host.InboundTag, ConfigProfileUUID: host.ConfigProfileUUID, ConfigProfileInboundUUID: host.ConfigProfileInboundUUID,
		IsDisabled: host.IsDisabled, IsHidden: host.IsHidden, ViewPosition: host.ViewPosition, Remark: host.Remark,
		VlessRouteID: host.VlessRouteID, RawInbound: host.RawInbound,
	}
	return r
}

// createFallbackHosts ports createFallbackHosts: a remark that is itself a
// serialized resolved config ('{"f…') is used as-is.
func createFallbackHosts(remarks []string) []*Resolved {
	hosts := make([]*Resolved, 0, len(remarks))
	for _, remark := range remarks {
		if parsed := parseResolvedFromRemark(strings.TrimSpace(remark)); parsed != nil {
			hosts = append(hosts, parsed)
			continue
		}
		hosts = append(hosts, &Resolved{
			FinalRemark: remark, Address: "0.0.0.0", Port: 1,
			Protocol: "vless", ID: "00000000-0000-0000-0000-000000000000", Encryption: "none",
			Transport: "tcp", TCP: &TCPOptions{}, Security: "none", Mapper: ojson.New(),
			Meta: Metadata{UUID: "00000000-0000-0000-0000-000000000000", Tags: []string{}, ExcludeFromSubscriptionTypes: []string{}, Remark: remark},
		})
	}
	return hosts
}
