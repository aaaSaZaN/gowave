package subscription

import (
	"fmt"
	"math"
	"strconv"

	"encoding/json"
	"strings"

	"remnawave-go/internal/database"

	"gopkg.in/yaml.v3"
)

type mihomoFieldMap struct {
	src      string
	dst      string
	asString bool
}

var xhttpFieldMaps = []mihomoFieldMap{
	{"noGRPCHeader", "no-grpc-header", false},
	{"xPaddingBytes", "x-padding-bytes", true},
	{"xPaddingObfsMode", "x-padding-obfs-mode", false},
	{"xPaddingKey", "x-padding-key", false},
	{"xPaddingHeader", "x-padding-header", false},
	{"xPaddingPlacement", "x-padding-placement", false},
	{"xPaddingMethod", "x-padding-method", false},
	{"uplinkHTTPMethod", "uplink-http-method", false},
	{"sessionIDPlacement", "session-placement", false},
	{"sessionIDKey", "session-key", false},
	{"sessionIDTable", "session-table", false},
	{"sessionIDLength", "session-length", true},
	{"seqPlacement", "seq-placement", false},
	{"seqKey", "seq-key", false},
	{"uplinkDataPlacement", "uplink-data-placement", false},
	{"uplinkDataKey", "uplink-data-key", false},
	{"uplinkChunkSize", "uplink-chunk-size", false},
	{"scMaxEachPostBytes", "sc-max-each-post-bytes", false},
	{"scMinPostsIntervalMs", "sc-min-posts-interval-ms", false},
	{"scStreamUpServerSecs", "sc-stream-up-server-secs", true},
}

var xmuxFieldMaps = []mihomoFieldMap{
	{"maxConnections", "max-connections", true},
	{"maxConcurrency", "max-concurrency", true},
	{"cMaxReuseTimes", "c-max-reuse-times", true},
	{"hMaxRequestTimes", "h-max-request-times", true},
	{"hMaxReusableSecs", "h-max-reusable-secs", true},
	{"hKeepAlivePeriod", "h-keep-alive-period", false},
}

func applyMihomoFieldMap(source map[string]interface{}, target map[string]interface{}, fieldMap []mihomoFieldMap) {
	for _, mapping := range fieldMap {
		if val, ok := source[mapping.src]; ok && val != nil {
			if mapping.asString {
				switch v := val.(type) {
				case float64:
					target[mapping.dst] = fmt.Sprintf("%.0f", v)
				case int, int32, int64:
					target[mapping.dst] = fmt.Sprintf("%d", v)
				default:
					target[mapping.dst] = fmt.Sprintf("%v", val)
				}
			} else {
				switch v := val.(type) {
				case float64:
					if v == math.Trunc(v) && !math.IsNaN(v) && !math.IsInf(v, 0) {
						target[mapping.dst] = int64(v)
					} else {
						target[mapping.dst] = v
					}
				case string:
					if !strings.Contains(v, "-") && strings.ContainsAny(strings.ToLower(v), "e") {
						if f, err := strconv.ParseFloat(v, 64); err == nil && f == math.Trunc(f) && !math.IsNaN(f) && !math.IsInf(f, 0) {
							target[mapping.dst] = int64(f)
							continue
						}
					}
					target[mapping.dst] = v
				default:
					target[mapping.dst] = val
				}
			}
		}
	}
}

func buildMihomoXhttpDownloadSettings(ds map[string]interface{}, hm XrayHostMeta) map[string]interface{} {
	settings := map[string]interface{}{}

	if address, ok := ds["address"].(string); ok && address != "" {
		settings["server"] = address
	}
	if port, ok := ds["port"].(float64); ok {
		settings["port"] = int(port)
	}

	if sec, ok := ds["security"].(string); ok && (sec == "tls" || sec == "reality") {
		settings["tls"] = true

		if tlsRaw, ok := ds["tlsSettings"].(map[string]interface{}); ok {
			if sn, ok := tlsRaw["serverName"].(string); ok && sn != "" {
				settings["servername"] = sn
			}
			if fp, ok := tlsRaw["fingerprint"].(string); ok && fp != "" {
				settings["client-fingerprint"] = fp
			}
			if alpn, ok := tlsRaw["alpn"].([]interface{}); ok && len(alpn) > 0 {
				settings["alpn"] = alpn
			}
			if allowInsecure, ok := tlsRaw["allowInsecure"].(bool); ok && allowInsecure {
				settings["skip-cert-verify"] = true
			}
		}

		if sec == "reality" {
			if realRaw, ok := ds["realitySettings"].(map[string]interface{}); ok {
				realityOpts := map[string]interface{}{}
				if pk, ok := realRaw["publicKey"].(string); ok && pk != "" {
					realityOpts["public-key"] = pk
				}
				if shortId, ok := realRaw["shortId"].(string); ok && shortId != "" {
					realityOpts["short-id"] = shortId
				}

				if hm.Host != nil && hm.Host.MihomoX25519 {
					realityOpts["support-x25519mlkem768"] = true
				}

				if len(realityOpts) > 0 {
					settings["reality-opts"] = realityOpts
				}
			}
		}
	} else if sec, ok := ds["security"].(string); ok && sec != "none" && sec != "" {
		// unsupported security maybe
	}

	// Also path and host
	if path, ok := ds["path"].(string); ok && path != "" {
		settings["path"] = path
	}
	if host, ok := ds["host"].(string); ok && host != "" {
		settings["host"] = host
	}
	if headers, ok := ds["headers"].(map[string]interface{}); ok {
		settings["headers"] = headers
	}

	if extra, ok := ds["extra"].(map[string]interface{}); ok {
		applyMihomoFieldMap(extra, settings, xhttpFieldMaps)
		if xmuxRaw, ok := extra["xmux"]; ok && xmuxRaw != nil {
			if xmux, ok := xmuxRaw.(map[string]interface{}); ok {
				reuseSettings := map[string]interface{}{}
				applyMihomoFieldMap(xmux, reuseSettings, xmuxFieldMaps)
				if len(reuseSettings) > 0 {
					settings["reuse-settings"] = reuseSettings
				}
			}
		}
	}

	return settings
}

func GenerateMihomoYAML(user *database.User, hosts []XrayHostMeta, templateYaml string, targetType string) (string, error) {
	if targetType == "" {
		targetType = "MIHOMO"
	}

	var proxyNodes []map[string]interface{}
	var proxyRemarks []string

	for _, hm := range hosts {
		h := hm.Host
		if h.IsDisabled || h.IsHidden {
			continue
		}
		if IsExcluded(h.ExcludeFromSubscriptionTypes, targetType) {
			continue
		}

		node := buildMihomoProxyNode(user, hm)
		if node != nil {
			// Stash does not support xhttp transport
			if targetType == "STASH" {
				if net, ok := node["network"].(string); ok && strings.ToLower(net) == "xhttp" {
					continue
				}
			}
			proxyNodes = append(proxyNodes, node)
			proxyRemarks = append(proxyRemarks, h.Remark)
		}
	}

	var root yaml.Node
	if templateYaml != "" {
		if err := yaml.Unmarshal([]byte(templateYaml), &root); err != nil {
			root = yaml.Node{}
		}
	}

	// If template is empty or invalid, construct default template
	if len(root.Content) == 0 {
		defaultYAML := `mixed-port: 7890
allow-lan: false
mode: rule
log-level: info
proxies: []
proxy-groups:
  - name: PROXY
    type: select
    proxies: []
rules:
  - MATCH,PROXY
`
		_ = yaml.Unmarshal([]byte(defaultYAML), &root)
	}

	// Process root node (mapping)
	var mappingNode *yaml.Node
	if root.Kind == yaml.DocumentNode && len(root.Content) > 0 {
		mappingNode = root.Content[0]
	} else if root.Kind == yaml.MappingNode {
		mappingNode = &root
	}

	if mappingNode != nil && mappingNode.Kind == yaml.MappingNode {
		// Serialize generated proxies to YAML nodes
		var proxiesSeq *yaml.Node
		if len(proxyNodes) > 0 {
			proxiesYAMLBytes, _ := yaml.Marshal(proxyNodes)
			var tmpDoc yaml.Node
			_ = yaml.Unmarshal(proxiesYAMLBytes, &tmpDoc)
			if tmpDoc.Kind == yaml.DocumentNode && len(tmpDoc.Content) > 0 {
				proxiesSeq = tmpDoc.Content[0]
			}
		} else {
			proxiesSeq = &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		}

		var proxiesValNode *yaml.Node
		var proxyGroupsValNode *yaml.Node
		var remnawaveKeyIdx = -1

		for i := 0; i < len(mappingNode.Content); i += 2 {
			k := mappingNode.Content[i].Value
			if k == "proxies" {
				proxiesValNode = mappingNode.Content[i+1]
			} else if k == "proxy-groups" {
				proxyGroupsValNode = mappingNode.Content[i+1]
			} else if k == "remnawave" {
				remnawaveKeyIdx = i
			}
		}

		// Remove root remnawave key if present
		if remnawaveKeyIdx != -1 {
			mappingNode.Content = append(mappingNode.Content[:remnawaveKeyIdx], mappingNode.Content[remnawaveKeyIdx+2:]...)
		}

		// Set proxies
		if proxiesValNode != nil && proxiesSeq != nil {
			*proxiesValNode = *proxiesSeq
		} else if proxiesSeq != nil {
			keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: "proxies", Tag: "!!str"}
			mappingNode.Content = append([]*yaml.Node{keyNode, proxiesSeq}, mappingNode.Content...)
		}

		// Update proxy-groups
		if proxyGroupsValNode != nil && proxyGroupsValNode.Kind == yaml.SequenceNode {
			for _, grpNode := range proxyGroupsValNode.Content {
				if grpNode.Kind != yaml.MappingNode {
					continue
				}

				var grpProxiesValNode *yaml.Node
				var remnawaveGrpIdx = -1
				var includeProxies = true
				var selectRandomProxy = false
				var shuffleProxies = false

				for i := 0; i < len(grpNode.Content); i += 2 {
					gk := grpNode.Content[i].Value
					if gk == "proxies" {
						grpProxiesValNode = grpNode.Content[i+1]
					} else if gk == "remnawave" {
						remnawaveGrpIdx = i
						remNode := grpNode.Content[i+1]
						if remNode.Kind == yaml.MappingNode {
							for j := 0; j < len(remNode.Content); j += 2 {
								rk := remNode.Content[j].Value
								rv := remNode.Content[j+1].Value
								if rk == "include-proxies" && rv == "false" {
									includeProxies = false
								} else if rk == "select-random-proxy" && rv == "true" {
									selectRandomProxy = true
								} else if rk == "shuffle-proxies-order" && rv == "true" {
									shuffleProxies = true
								}
							}
						}
					}
				}

				// Remove remnawave metadata from group
				if remnawaveGrpIdx != -1 {
					grpNode.Content = append(grpNode.Content[:remnawaveGrpIdx], grpNode.Content[remnawaveGrpIdx+2:]...)
				}

				if !includeProxies {
					continue
				}

				selectedRemarks := make([]string, len(proxyRemarks))
				copy(selectedRemarks, proxyRemarks)

				if shuffleProxies {
					// Deterministic stable order or simple reverse
				}
				if selectRandomProxy && len(selectedRemarks) > 1 {
					selectedRemarks = selectedRemarks[:1]
				}

				if grpProxiesValNode != nil && grpProxiesValNode.Kind == yaml.SequenceNode {
					// Append proxy remarks
					for _, rem := range selectedRemarks {
						grpProxiesValNode.Content = append(grpProxiesValNode.Content, &yaml.Node{
							Kind:  yaml.ScalarNode,
							Value: rem,
							Tag:   "!!str",
						})
					}
				} else if grpProxiesValNode != nil {
					// Empty/null proxies in group
					grpProxiesValNode.Kind = yaml.SequenceNode
					grpProxiesValNode.Tag = "!!seq"
					grpProxiesValNode.Content = nil
					for _, rem := range selectedRemarks {
						grpProxiesValNode.Content = append(grpProxiesValNode.Content, &yaml.Node{
							Kind:  yaml.ScalarNode,
							Value: rem,
							Tag:   "!!str",
						})
					}
				}
			}
		}
	}

	outBytes, err := yaml.Marshal(&root)
	if err != nil {
		return "", err
	}
	return string(outBytes), nil
}

func buildMihomoProxyNode(user *database.User, hm XrayHostMeta) map[string]interface{} {
	h := hm.Host
	inb := hm.Inbound

	var rawInb XrayRawInbound
	var rawInbMap map[string]interface{}
	if inb != nil && inb.RawInbound != "" {
		_ = json.Unmarshal([]byte(inb.RawInbound), &rawInb)
		_ = json.Unmarshal([]byte(inb.RawInbound), &rawInbMap)
	}

	protocol := "vless"
	if inb != nil && inb.Type != "" {
		protocol = strings.ToLower(inb.Type)
	}

	network := "tcp"
	if inb != nil && inb.Network != nil && *inb.Network != "" {
		network = strings.ToLower(*inb.Network)
	}
	if rawInb.StreamSettings != nil && rawInb.StreamSettings.Network != "" {
		network = strings.ToLower(rawInb.StreamSettings.Network)
	}
	if network == "raw" {
		network = "tcp"
	}

	security := "none"
	if inb != nil && inb.Security != nil && *inb.Security != "" {
		security = strings.ToLower(*inb.Security)
	}
	if rawInb.StreamSettings != nil && rawInb.StreamSettings.Security != "" {
		security = strings.ToLower(rawInb.StreamSettings.Security)
	}

	// Host overrides for security
	if h.SecurityLayer != "" && h.SecurityLayer != "DEFAULT" {
		security = strings.ToLower(h.SecurityLayer)
	}

	serverName := h.Sni
	if serverName == "" && h.Address != "" {
		serverName = h.Address
	}

	fingerprint := h.Fingerprint
	if fingerprint == "" {
		fingerprint = "chrome"
	}

	// 1. Hysteria 2 Protocol
	if protocol == "hysteria" {
		node := map[string]interface{}{
			"name":             h.Remark,
			"type":             "hysteria2",
			"server":           h.Address,
			"port":             h.Port,
			"password":         user.VlessUUID,
			"sni":              serverName,
			"skip-cert-verify": false,
			"udp":              true,
		}

		// Salamander UDP Obfuscation
		var obfsPassword string
		if rawInbMap != nil {
			if ss, ok := rawInbMap["streamSettings"].(map[string]interface{}); ok {
				if fm, ok := ss["finalmask"].(map[string]interface{}); ok {
					if udpList, ok := fm["udp"].([]interface{}); ok && len(udpList) > 0 {
						if udp0, ok := udpList[0].(map[string]interface{}); ok {
							if udp0["type"] == "salamander" {
								if settings, ok := udp0["settings"].(map[string]interface{}); ok {
									if pw, ok := settings["password"].(string); ok {
										obfsPassword = pw
									}
								}
							}
						}
					}
				}
			}
		}
		if obfsPassword == "" && h.FinalMask != "" && h.FinalMask != "null" {
			var fm map[string]interface{}
			if err := json.Unmarshal([]byte(h.FinalMask), &fm); err == nil {
				if udpList, ok := fm["udp"].([]interface{}); ok && len(udpList) > 0 {
					if udp0, ok := udpList[0].(map[string]interface{}); ok {
						if udp0["type"] == "salamander" {
							if settings, ok := udp0["settings"].(map[string]interface{}); ok {
								if pw, ok := settings["password"].(string); ok {
									obfsPassword = pw
								}
							}
						}
					}
				}
			}
		}

		if obfsPassword != "" {
			node["obfs"] = "salamander"
			node["obfs-password"] = obfsPassword
		}

		return node
	}

	// 2. VLESS Protocol
	if protocol == "vless" {
		node := map[string]interface{}{
			"name":            h.Remark,
			"type":            "vless",
			"server":          h.Address,
			"port":            h.Port,
			"uuid":            user.VlessUUID,
			"udp":             true,
			"packet-encoding": "xudp",
		}

		if network != "tcp" {
			node["network"] = network
		}

		// Flow
		flow := ResolveVlessFlow(rawInbMap, network, security)
		if flow != "" && flow != "none" {
			node["flow"] = flow
		}
		// Encryption
		encryption := "none"
		if rawInbMap != nil {
			if settingsMap, ok := rawInbMap["settings"].(map[string]interface{}); ok {
				if dec, ok := settingsMap["decryption"].(string); ok && dec != "" && dec != "none" {
					if enc, err := DeriveVlessEncryption(dec); err == nil && enc != "" {
						encryption = enc
					}
				}
			}
		}
		if encryption != "" && encryption != "none" {
			node["encryption"] = encryption
		}

		// Security: Reality
		if security == "reality" {
			node["tls"] = true
			node["servername"] = serverName
			node["client-fingerprint"] = fingerprint

			var realityCfg XrayRawRealitySettings
			if rawInb.StreamSettings != nil && rawInb.StreamSettings.RealitySettings != nil {
				realityCfg = *rawInb.StreamSettings.RealitySettings
			}

			if serverName == "" && len(realityCfg.ServerNames) > 0 {
				node["servername"] = realityCfg.ServerNames[0]
			}

			publicKey := ""
			if realityCfg.PrivateKey != "" {
				if pk, err := DeriveX25519PublicKey(realityCfg.PrivateKey); err == nil {
					publicKey = pk
				}
			}

			shortID := ""
			if len(realityCfg.ShortIds) > 0 {
				shortID = realityCfg.ShortIds[0]
			}

			realityOpts := map[string]interface{}{
				"public-key": publicKey,
				"short-id":   shortID,
			}
			if hm.Host != nil && hm.Host.MihomoX25519 {
				realityOpts["support-x25519mlkem768"] = true
			}
			node["reality-opts"] = realityOpts

		} else if security == "tls" {
			node["tls"] = true
			node["servername"] = serverName
			node["client-fingerprint"] = fingerprint

			if h.Alpn != "" {
				var alpnList []string
				for _, a := range strings.Split(h.Alpn, ",") {
					a = strings.TrimSpace(a)
					if a != "" {
						alpnList = append(alpnList, a)
					}
				}
				if len(alpnList) > 0 {
					node["alpn"] = alpnList
				}
			}
		}

		// Transports
		if network == "ws" {
			wsOpts := map[string]interface{}{}
			path := h.Path
			if path == "" && rawInb.StreamSettings != nil && rawInb.StreamSettings.WsSettings != nil {
				if p, ok := rawInb.StreamSettings.WsSettings["path"].(string); ok {
					path = p
				}
			}
			if path != "" {
				wsOpts["path"] = path
			}
			host := h.Host
			if host == "" && rawInb.StreamSettings != nil && rawInb.StreamSettings.WsSettings != nil {
				if hdrs, ok := rawInb.StreamSettings.WsSettings["headers"].(map[string]interface{}); ok {
					if hStr, ok := hdrs["Host"].(string); ok {
						host = hStr
					}
				}
			}
			var headers map[string]interface{}
			if rawInbMap != nil {
				if stMap, ok := rawInbMap["streamSettings"].(map[string]interface{}); ok {
					if xhMap, ok := stMap["xhttpSettings"].(map[string]interface{}); ok {
						if hdrs, ok := xhMap["headers"].(map[string]interface{}); ok {
							headers = hdrs
						}
					}
				}
			}
			if host == "" && headers != nil {
				if hStr, ok := headers["Host"].(string); ok {
					host = hStr
				}
			}

			if host != "" {
				wsOpts["headers"] = map[string]interface{}{
					"Host": host,
				}
			}
			if len(wsOpts) > 0 {
				node["ws-opts"] = wsOpts
			}
		} else if network == "xhttp" {
			xhttpOpts := map[string]interface{}{}
			path := h.Path
			if path == "" && rawInb.StreamSettings != nil && rawInb.StreamSettings.XhttpSettings != nil {
				if p, ok := rawInb.StreamSettings.XhttpSettings["path"].(string); ok {
					path = p
				}
			}
			if path != "" {
				xhttpOpts["path"] = path
			}
			host := h.Host
			if host == "" && rawInb.StreamSettings != nil && rawInb.StreamSettings.XhttpSettings != nil {
				if hStr, ok := rawInb.StreamSettings.XhttpSettings["host"].(string); ok {
					host = hStr
				}
			}
			var headers map[string]interface{}
			if rawInbMap != nil {
				if stMap, ok := rawInbMap["streamSettings"].(map[string]interface{}); ok {
					if xhMap, ok := stMap["xhttpSettings"].(map[string]interface{}); ok {
						if hdrs, ok := xhMap["headers"].(map[string]interface{}); ok {
							headers = hdrs
						}
					}
				}
			}
			if host == "" && headers != nil {
				if hStr, ok := headers["Host"].(string); ok {
					host = hStr
				}
			}

			if host != "" {
				xhttpOpts["host"] = host
			}
			if headers != nil && len(headers) > 0 {
				xhttpOpts["headers"] = headers

			}

			mode := "auto"
			if rawInb.StreamSettings != nil && rawInb.StreamSettings.XhttpSettings != nil {
				if m, ok := rawInb.StreamSettings.XhttpSettings["mode"].(string); ok && m != "" {
					mode = m
				}
			}
			xhttpOpts["mode"] = mode

			var extra map[string]interface{}
			if rawInbMap != nil {
				if stMap, ok := rawInbMap["streamSettings"].(map[string]interface{}); ok {
					if xhMap, ok := stMap["xhttpSettings"].(map[string]interface{}); ok {
						if ex, ok := xhMap["extra"].(map[string]interface{}); ok {
							extra = ex
						}
					}
				}
			}
			if h.XhttpExtraParams != "" && h.XhttpExtraParams != "{}" && h.XhttpExtraParams != "null" {
				var hostExtra map[string]interface{}
				if err := json.Unmarshal([]byte(h.XhttpExtraParams), &hostExtra); err == nil {
					extra = hostExtra
				}
			}
			if extra != nil {
				applyMihomoFieldMap(extra, xhttpOpts, xhttpFieldMaps)

				if xmuxRaw, ok := extra["xmux"]; ok && xmuxRaw != nil {
					if xmux, ok := xmuxRaw.(map[string]interface{}); ok {
						reuseSettings := map[string]interface{}{}
						applyMihomoFieldMap(xmux, reuseSettings, xmuxFieldMaps)
						if len(reuseSettings) > 0 {
							xhttpOpts["reuse-settings"] = reuseSettings
						}
					}
				}

				if dsRaw, ok := extra["downloadSettings"]; ok && dsRaw != nil {
					if dsMap, ok := dsRaw.(map[string]interface{}); ok {
						dsConfig := buildMihomoXhttpDownloadSettings(dsMap, hm)
						if len(dsConfig) > 0 {
							xhttpOpts["download-settings"] = dsConfig
						}
					}
				}
			}

			node["xhttp-opts"] = xhttpOpts
		} else if network == "grpc" {
			serviceName := h.Path
			if serviceName == "" && rawInb.StreamSettings != nil && rawInb.StreamSettings.GrpcSettings != nil {
				if sn, ok := rawInb.StreamSettings.GrpcSettings["serviceName"].(string); ok {
					serviceName = sn
				}
			}
			if serviceName != "" {
				node["grpc-opts"] = map[string]interface{}{
					"grpc-service-name": serviceName,
				}
			}
		}

		return node
	}

	// 3. Trojan
	if protocol == "trojan" {
		node := map[string]interface{}{
			"name":               h.Remark,
			"type":               "trojan",
			"server":             h.Address,
			"port":               h.Port,
			"password":           user.VlessUUID,
			"udp":                true,
			"tls":                true,
			"sni":                serverName,
			"client-fingerprint": fingerprint,
		}
		if network != "tcp" {
			node["network"] = network
		}
		return node
	}

	// 4. Shadowsocks
	if protocol == "shadowsocks" {
		method := "aes-256-gcm"
		if inb != nil && inb.Network != nil && *inb.Network != "" {
			method = *inb.Network
		}
		node := map[string]interface{}{
			"name":     h.Remark,
			"type":     "ss",
			"server":   h.Address,
			"port":     h.Port,
			"cipher":   method,
			"password": user.VlessUUID,
			"udp":      true,
		}
		return node
	}

	return nil
}
