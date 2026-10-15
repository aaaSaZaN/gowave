package subtpl

import (
	"math"
	"strings"

	"remnawave-go/internal/subtpl/js"
	"remnawave-go/internal/subtpl/ojson"
)

// GenerateLinks ports XrayGeneratorService.generateConfig.
func GenerateLinks(hosts []*Resolved, isBase64, isExtendedClient bool) string {
	joined := strings.Join(LinkList(hosts, isExtendedClient), "\n")
	if isBase64 {
		return js.Base64(joined)
	}
	return joined
}

// LinkList ports XrayGeneratorService.generateLinks.
func LinkList(hosts []*Resolved, isExtendedClient bool) []string {
	links := []string{}
	for _, host := range hosts {
		if contains(host.Meta.ExcludeFromSubscriptionTypes, "XRAY_BASE64") {
			continue
		}
		link := buildShareLink(host)
		if link == nil {
			continue
		}
		serialized := serializeLink(applyBase64Mapper(link, host))
		if isExtendedClient && host.ServerDescription != nil && *host.ServerDescription != "" {
			links = append(links, serialized+"?serverDescription="+*host.ServerDescription)
		} else {
			links = append(links, serialized)
		}
	}
	return links
}

func contains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}

// shareLink is the ShareLink object; params keep insertion order.
type shareLink = ojson.Object

func buildShareLink(host *Resolved) *shareLink {
	switch host.Protocol {
	case "vless":
		params := ojson.New()
		if host.Encryption != "" {
			params.Set("encryption", host.Encryption)
		}
		if host.Flow != "" {
			params.Set("flow", host.Flow)
		}
		applyTransportParams(params, host)
		applySecurityParams(params, host)
		if host.FinalMask != nil {
			params.Set("fm", ojson.Stringify(host.FinalMask))
		}
		return ojson.New("scheme", "vless", "password", host.ID, "address", host.Address, "port", float64(host.Port), "remark", host.FinalRemark, "params", params)
	case "trojan":
		params := ojson.New()
		applyTransportParams(params, host)
		applySecurityParams(params, host)
		return ojson.New("scheme", "trojan", "password", host.Password, "address", host.Address, "port", float64(host.Port), "remark", host.FinalRemark, "params", params)
	case "shadowsocks":
		return ojson.New("scheme", "ss", "method", host.Method, "password", host.Password, "address", host.Address, "port", float64(host.Port), "remark", host.FinalRemark, "params", ojson.New())
	case "hysteria":
		if host.Transport != "hysteria" {
			return nil
		}
		params := ojson.New()
		if udp, ok := get(host.FinalMask, "udp").([]any); ok {
			for _, mask := range udp {
				if get(mask, "type") == "salamander" {
					if password := get(mask, "settings", "password"); ojson.Truthy(password) {
						params.Set("obfs", "salamander")
						params.Set("obfs-password", password)
					}
					break
				}
			}
		}
		if host.Security == "tls" {
			if host.TLS.ServerName != "" {
				params.Set("sni", host.TLS.ServerName)
			}
			if p := host.TLS.PinnedPeerCertSha256; p != nil && *p != "" {
				params.Set("pinSHA256", *p)
			}
		}
		if host.FinalMask != nil {
			params.Set("fm", ojson.Stringify(host.FinalMask))
		}
		return ojson.New("scheme", "hysteria2", "password", host.Hysteria.Auth, "address", host.Address, "port", float64(host.Port), "remark", host.FinalRemark, "params", params)
	}
	return nil
}

func setTruthy(params *ojson.Object, key string, value any) {
	if ojson.Truthy(value) {
		params.Set(key, value)
	}
}

func applyTransportParams(params *ojson.Object, host *Resolved) {
	params.Set("type", host.Transport)
	switch host.Transport {
	case "tcp":
		header, _ := host.TCP.Header.(*ojson.Object)
		if !ojson.Truthy(host.TCP.Header) {
			return
		}
		params.Set("headerType", get(header, "type"))
		if get(header, "type") != "http" || !ojson.Truthy(get(header, "request")) {
			return
		}
		params.Set("path", joinOrEmpty(get(header, "request", "path")))
		params.Set("host", joinOrEmpty(get(header, "request", "headers", "Host")))
	case "ws":
		setTruthy(params, "path", ptrAny(host.WS.Path))
		setTruthy(params, "host", host.WS.Host)
		setTruthy(params, "heartbeatPeriod", host.WS.HeartbeatPeriod)
	case "httpupgrade":
		setTruthy(params, "path", ptrAny(host.HTTPUp.Path))
		setTruthy(params, "host", host.HTTPUp.Host)
	case "grpc":
		setTruthy(params, "serviceName", ptrAny(host.GRPC.ServiceName))
		if host.GRPC.MultiMode {
			params.Set("mode", "multi")
		} else {
			params.Set("mode", "gun")
		}
		setTruthy(params, "authority", host.GRPC.Authority)
	case "xhttp":
		setTruthy(params, "path", ptrAny(host.XHTTP.Path))
		setTruthy(params, "host", host.XHTTP.Host)
		setTruthy(params, "mode", host.XHTTP.Mode)
		if host.XHTTP.Extra != nil {
			params.Set("extra", ojson.Stringify(host.XHTTP.Extra))
		}
	case "kcp":
		setTruthy(params, "mtu", host.KCP.ClientMtu)
		setTruthy(params, "tti", host.KCP.ClientTti)
	}
}

// joinOrEmpty is value?.join(',') ?? ” for arrays.
func joinOrEmpty(value any) any {
	if list, ok := value.([]any); ok {
		return jsString(list)
	}
	if value == nil {
		return ""
	}
	return value
}

func applySecurityParams(params *ojson.Object, host *Resolved) {
	params.Set("security", host.Security)
	switch host.Security {
	case "tls":
		o := host.TLS
		params.Set("sni", o.ServerName)
		setTruthy(params, "fp", o.Fingerprint)
		setTruthy(params, "alpn", o.ALPN)
		setTruthy(params, "pcs", ptrAny(o.PinnedPeerCertSha256))
		setTruthy(params, "vcn", ptrAny(o.VerifyPeerCertByName))
		setTruthy(params, "cs", o.CipherSuites)
	case "reality":
		o := host.Reality
		params.Set("sni", o.ServerName)
		if o.Fingerprint != "" {
			params.Set("fp", o.Fingerprint)
		} else {
			params.Set("fp", "chrome")
		}
		setTruthy(params, "pbk", o.PublicKey)
		setTruthy(params, "sid", o.ShortID)
		setTruthy(params, "pqv", ptrAny(o.Mldsa65Verify))
		setTruthy(params, "spx", o.SpiderX)
	}
}

var linkAllowedTargets = map[string]bool{"address": true, "method": true, "password": true, "port": true, "remark": true}

func applyBase64Mapper(link *shareLink, host *Resolved) *shareLink {
	operations := host.mapperOps("base64")
	if len(operations) == 0 {
		return link
	}
	var linkOps, queryOps []any
	for _, raw := range operations {
		op, ok := raw.(*ojson.Object)
		if !ok {
			continue
		}
		to, _ := op.Value("to").(string)
		if !strings.HasPrefix(to, "$link.") {
			queryOps = append(queryOps, op)
			continue
		}
		target := strings.TrimPrefix(to, "$link.")
		if !linkAllowedTargets[target] {
			continue
		}
		linkOps = append(linkOps, ojson.New().Spread(op).Set("to", target))
	}
	mapped := applyHostMapper(link, linkOps, host, true).(*ojson.Object)
	mapped.Set("params", applyHostMapper(link.Value("params"), queryOps, host, true))
	return sanitizeLink(mapped, link)
}

func sanitizeLink(mapped, original *shareLink) *shareLink {
	address := strings.TrimSpace(nullToEmpty(mapped.Value("address")))
	port := jsNumber(mapped.Value("port"))
	password := nullToEmpty(mapped.Value("password"))
	result := ojson.New().Spread(mapped)
	result.Set("scheme", original.Value("scheme"))
	if address == "" {
		result.Set("address", original.Value("address"))
	} else {
		result.Set("address", address)
	}
	if port == math.Trunc(port) && port > 0 && port <= 65535 {
		result.Set("port", port)
	} else {
		result.Set("port", original.Value("port"))
	}
	if password == "" {
		result.Set("password", original.Value("password"))
	} else {
		result.Set("password", password)
	}
	method, hasMethod := mapped.Get("method")
	if hasMethod && method != ojson.Undefined && jsString(method) != "" {
		result.Set("method", jsString(method))
	} else {
		result.Set("method", original.Value("method"))
		if !original.Has("method") {
			result.Set("method", ojson.Undefined)
		}
	}
	remark, hasRemark := mapped.Get("remark")
	if !hasRemark || remark == nil || remark == ojson.Undefined {
		result.Set("remark", original.Value("remark"))
	} else {
		result.Set("remark", jsString(remark))
	}
	return result
}

// nullToEmpty is String(value ?? ”).
func nullToEmpty(value any) string {
	if value == nil || value == ojson.Undefined {
		return ""
	}
	return jsString(value)
}

// jsNumber is Number(value).
func jsNumber(value any) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case bool:
		if v {
			return 1
		}
		return 0
	case nil:
		return 0
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return 0
		}
		parsed, err := ojson.Parse([]byte(trimmed))
		if f, ok := parsed.(float64); err == nil && ok {
			return f
		}
	}
	return math.NaN()
}

func serializeLink(link *shareLink) string {
	params, _ := link.Value("params").(*ojson.Object)
	var parts []string
	for _, key := range params.Keys() {
		value := params.Value(key)
		if value == nil || value == ojson.Undefined {
			continue
		}
		parts = append(parts, js.EncodeURIComponent(key)+"="+js.EncodeURIComponent(jsString(value)))
	}
	query := strings.Join(parts, "&")
	remark := js.EncodeURIComponent(jsString(link.Value("remark")))
	address := jsString(link.Value("address"))
	if strings.Contains(address, ":") && !strings.HasPrefix(address, "[") {
		address = "[" + address + "]"
	}
	port := jsString(link.Value("port"))
	password := jsString(link.Value("password"))
	switch link.Value("scheme") {
	case "ss":
		credentials := js.Base64(jsString(link.Value("method")) + ":" + password)
		return "ss://" + credentials + "@" + address + ":" + port + "#" + remark
	case "hysteria2":
		queryPart := ""
		if query != "" {
			queryPart = "?" + query
		}
		return "hysteria2://" + js.EncodeURIComponent(password) + "@" + address + ":" + port + "/" + queryPart + "#" + remark
	}
	return jsString(link.Value("scheme")) + "://" + js.EncodeURIComponent(password) + "@" + address + ":" + port + "?" + query + "#" + remark
}
