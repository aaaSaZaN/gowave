package subscription

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"remnawave-go/internal/database"
)

type XrayHostMeta struct {
	Host    *database.Host
	Inbound *database.ConfigProfileInbound
}

type XrayRawStreamSettings struct {
	Network         string                  `json:"network"`
	Security        string                  `json:"security"`
	RealitySettings *XrayRawRealitySettings `json:"realitySettings,omitempty"`
	TLSSettings     *XrayRawTLSSettings     `json:"tlsSettings,omitempty"`
	WsSettings      map[string]interface{}  `json:"wsSettings,omitempty"`
	XhttpSettings   map[string]interface{}  `json:"xhttpSettings,omitempty"`
	TCPSettings     map[string]interface{}  `json:"tcpSettings,omitempty"`
	GrpcSettings    map[string]interface{}  `json:"grpcSettings,omitempty"`
}

type XrayRawRealitySettings struct {
	Show        bool     `json:"show"`
	PrivateKey  string   `json:"privateKey,omitempty"`
	ShortIds    []string `json:"shortIds,omitempty"`
	ServerNames []string `json:"serverNames,omitempty"`
	Fingerprint string   `json:"fingerprint,omitempty"`
	SpiderX     string   `json:"spiderX,omitempty"`
}

type XrayRawTLSSettings struct {
	ServerName  string   `json:"serverName,omitempty"`
	Fingerprint string   `json:"fingerprint,omitempty"`
	Alpn        []string `json:"alpn,omitempty"`
}

type XrayRawInbound struct {
	Tag            string                 `json:"tag"`
	Protocol       string                 `json:"protocol"`
	Settings       map[string]interface{} `json:"settings,omitempty"`
	StreamSettings *XrayRawStreamSettings `json:"streamSettings,omitempty"`
}

func GenerateXrayJSON(user *database.User, hosts []XrayHostMeta, defaultTemplateJSON string, templateResolver func(h *database.Host) string) (string, error) {
	var fallbackTemplate map[string]interface{}
	if defaultTemplateJSON != "" && defaultTemplateJSON != "{}" && defaultTemplateJSON != "null" {
		_ = json.Unmarshal([]byte(defaultTemplateJSON), &fallbackTemplate)
	}
	if fallbackTemplate == nil {
		fallbackTemplate = map[string]interface{}{
			"log": map[string]interface{}{
				"loglevel": "warning",
			},
			"inbounds": []interface{}{
				map[string]interface{}{
					"listen":   "127.0.0.1",
					"port":     10808,
					"protocol": "socks",
					"sniffing": map[string]interface{}{
						"enabled":      true,
						"destOverride": []string{"http", "tls"},
					},
				},
			},
			"outbounds": []interface{}{
				map[string]interface{}{
					"protocol": "freedom",
					"tag":      "direct",
				},
			},
		}
	}

	var configs []map[string]interface{}

	for _, hm := range hosts {
		h := hm.Host
		if h.IsDisabled || h.IsHidden {
			continue
		}
		if IsExcluded(h.ExcludeFromSubscriptionTypes, "XRAY_JSON") {
			continue
		}

		// Resolve template for this specific host
		hostTmpl := defaultTemplateJSON
		if templateResolver != nil {
			t := templateResolver(h)
			if t != "" && t != "{}" && t != "null" {
				hostTmpl = t
			}
		}

		var hostBase map[string]interface{}
		if hostTmpl != "" && hostTmpl != "{}" && hostTmpl != "null" {
			_ = json.Unmarshal([]byte(hostTmpl), &hostBase)
		}
		if hostBase == nil {
			hostBase = fallbackTemplate
		}

		// Clone base template
		cfg := make(map[string]interface{})
		for k, v := range hostBase {
			cfg[k] = v
		}

		// Check if template contains remnawave injector
		remnawaveVal, hasRemnawave := cfg["remnawave"]
		delete(cfg, "remnawave") // never leak internal remnawave key in output JSON

		if hasRemnawave && remnawaveVal != nil {
			if remMap, ok := remnawaveVal.(map[string]interface{}); ok {
				injectedOutbounds := applyRemnawaveInjector(user, hm, hosts, remMap)
				existingOutbounds, _ := cfg["outbounds"].([]interface{})
				cfg["outbounds"] = append(injectedOutbounds, existingOutbounds...)
				cfg["remarks"] = h.Remark
				configs = append(configs, cfg)
				continue
			}
		}

		// Normal host
		outbound := buildOutbound(user, hm, "proxy")
		if outbound == nil {
			continue
		}

		existingOutbounds, _ := cfg["outbounds"].([]interface{})
		newOutbounds := append([]interface{}{outbound}, existingOutbounds...)
		cfg["outbounds"] = newOutbounds
		cfg["remarks"] = h.Remark

		configs = append(configs, cfg)
	}

	resBytes, err := json.Marshal(configs)
	if err != nil {
		return "[]", err
	}
	return string(resBytes), nil
}

func applyRemnawaveInjector(user *database.User, currentHost XrayHostMeta, allHosts []XrayHostMeta, remMap map[string]interface{}) []interface{} {
	var outbounds []interface{}

	// addVirtualHostAsOutbound defaults to true unless explicitly false
	addVirtualHost := true
	if v, exists := remMap["addVirtualHostAsOutbound"]; exists {
		if b, ok := v.(bool); ok {
			addVirtualHost = b
		}
	}
	if addVirtualHost {
		if ob := buildOutbound(user, currentHost, "proxy"); ob != nil {
			outbounds = append(outbounds, ob)
		}
	}

	injectHostsList, _ := remMap["injectHosts"].([]interface{})
	for _, entry := range injectHostsList {
		entryMap, ok := entry.(map[string]interface{})
		if !ok {
			continue
		}
		selectorMap, _ := entryMap["selector"].(map[string]interface{})
		selType, _ := selectorMap["type"].(string)
		pattern, _ := selectorMap["pattern"].(string)
		tagPrefix, _ := entryMap["tagPrefix"].(string)
		if tagPrefix == "" {
			tagPrefix = "proxy"
		}
		selectFrom, _ := entryMap["selectFrom"].(string)
		if selectFrom == "" {
			selectFrom = "ALL"
		}
		useHostRemarkAsTag, _ := entryMap["useHostRemarkAsTag"].(bool)
		useHostTagAsTag, _ := entryMap["useHostTagAsTag"].(bool)

		var re *regexp.Regexp
		if pattern != "" {
			re, _ = regexp.Compile(pattern)
		}

		var matched []XrayHostMeta
		for _, cand := range allHosts {
			cH := cand.Host
			if cH.UUID == currentHost.Host.UUID {
				continue
			}
			switch selectFrom {
			case "HIDDEN":
				if !cH.IsHidden {
					continue
				}
			case "NOT_HIDDEN":
				if cH.IsHidden {
					continue
				}
			}

			isMatch := false
			candTags := ParseArray(cH.Tags)
			switch selType {
			case "tagRegex":
				if re != nil {
					for _, tg := range candTags {
						if re.MatchString(tg) {
							isMatch = true
							break
						}
					}
				}
			case "remarkRegex":
				if re != nil && re.MatchString(cH.Remark) {
					isMatch = true
				}
			case "sameTagAsRecipient":
				currTags := ParseArray(currentHost.Host.Tags)
				for _, ct := range currTags {
					for _, tg := range candTags {
						if strings.EqualFold(ct, tg) {
							isMatch = true
							break
						}
					}
					if isMatch {
						break
					}
				}
			case "uuids":
				if values, ok := selectorMap["values"].([]interface{}); ok {
					for _, v := range values {
						if vStr, ok := v.(string); ok && vStr == cH.UUID {
							isMatch = true
							break
						}
					}
				}
			}

			if isMatch {
				matched = append(matched, cand)
			}
		}

		for idx, m := range matched {
			tag := tagPrefix
			if useHostRemarkAsTag {
				tag = m.Host.Remark
			} else if useHostTagAsTag {
				mTags := ParseArray(m.Host.Tags)
				if len(mTags) > 0 {
					tag = mTags[0]
				} else {
					tag = m.Host.Remark
				}
			} else {
				if idx == 0 {
					tag = tagPrefix
				} else {
					tag = fmt.Sprintf("%s-%d", tagPrefix, idx+1)
				}
			}
			if ob := buildOutbound(user, m, tag); ob != nil {
				outbounds = append(outbounds, ob)
			}
		}
	}

	return outbounds
}

func buildOutbound(user *database.User, hm XrayHostMeta, tag string) map[string]interface{} {
	h := hm.Host
	inb := hm.Inbound

	if inb == nil && h.Address == "" {
		return nil
	}

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
	fingerprint := h.Fingerprint
	if fingerprint == "" {
		fingerprint = "chrome"
	}

	// 1. Hysteria Outbound
	if protocol == "hysteria" {
		if serverName == "" {
			serverName = h.Address
		}
		streamSettings := map[string]interface{}{
			"network":  "hysteria",
			"security": "tls",
			"tlsSettings": map[string]interface{}{
				"serverName":  serverName,
				"fingerprint": fingerprint,
				"alpn":        []string{"h3"},
			},
			"hysteriaSettings": map[string]interface{}{
				"version": 2,
				"auth":    user.VlessUUID,
			},
		}

		// Extract finalmask from rawInb or host
		var finalMaskMap map[string]interface{}
		if ss, ok := rawInbMap["streamSettings"].(map[string]interface{}); ok {
			if fm, ok := ss["finalmask"].(map[string]interface{}); ok {
				finalMaskMap = fm
			}
		}
		if len(finalMaskMap) == 0 && h.FinalMask != "" && h.FinalMask != "null" {
			_ = json.Unmarshal([]byte(h.FinalMask), &finalMaskMap)
		}
		if len(finalMaskMap) > 0 {
			streamSettings["finalmask"] = finalMaskMap
		}

		return map[string]interface{}{
			"tag":      tag,
			"protocol": "hysteria",
			"settings": map[string]interface{}{
				"address": h.Address,
				"port":    h.Port,
				"version": 2,
			},
			"streamSettings": streamSettings,
		}
	}

	// 2. VLESS / Other Protocols
	flow := ResolveVlessFlow(rawInbMap, network, security)

	// Derive client VLESS encryption from server inbound settings.decryption if present
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

	userUUID := user.VlessUUID
	if h.UUID == "00000000-0000-0000-0000-000000000000" {
		userUUID = "00000000-0000-0000-0000-000000000000"
	}

	userObj := map[string]interface{}{
		"id":         userUUID,
		"encryption": encryption,
	}
	if flow != "" && flow != "none" {
		userObj["flow"] = flow
	}

	settings := map[string]interface{}{
		"vnext": []interface{}{
			map[string]interface{}{
				"address": h.Address,
				"port":    h.Port,
				"users": []interface{}{
					userObj,
				},
			},
		},
	}

	streamSettings := map[string]interface{}{
		"network": network,
	}

	if security == "reality" {
		streamSettings["security"] = "reality"

		var realityCfg XrayRawRealitySettings
		if rawInb.StreamSettings != nil && rawInb.StreamSettings.RealitySettings != nil {
			realityCfg = *rawInb.StreamSettings.RealitySettings
		}

		if serverName == "" && len(realityCfg.ServerNames) > 0 {
			serverName = realityCfg.ServerNames[0]
		}
		if serverName == "" && h.Address != "" {
			serverName = h.Address
		}
		if fingerprint == "" && realityCfg.Fingerprint != "" {
			fingerprint = realityCfg.Fingerprint
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

		realityObj := map[string]interface{}{
			"show":        false,
			"fingerprint": fingerprint,
			"serverName":  serverName,
			"publicKey":   publicKey,
			"shortId":     shortID,
			"spiderX":     realityCfg.SpiderX,
		}
		streamSettings["realitySettings"] = realityObj

	} else if security == "tls" {
		streamSettings["security"] = "tls"

		if serverName == "" && h.Address != "" {
			serverName = h.Address
		}

		var alpnList []string
		if h.Alpn != "" {
			for _, a := range strings.Split(h.Alpn, ",") {
				a = strings.TrimSpace(a)
				if a != "" {
					alpnList = append(alpnList, a)
				}
			}
		} else if rawInb.StreamSettings != nil && rawInb.StreamSettings.TLSSettings != nil {
			alpnList = rawInb.StreamSettings.TLSSettings.Alpn
		}

		tlsObj := map[string]interface{}{
			"serverName":  serverName,
			"fingerprint": fingerprint,
		}
		if len(alpnList) > 0 {
			tlsObj["alpn"] = alpnList
		}
		streamSettings["tlsSettings"] = tlsObj
	} else {
		streamSettings["security"] = "none"
	}

	// Transport settings with fallback to rawInbound
	if network == "ws" {
		wsObj := map[string]interface{}{}
		path := h.Path
		hostHeader := h.Host
		var headers map[string]interface{}

		if rawInbMap != nil {
			if ss, ok := rawInbMap["streamSettings"].(map[string]interface{}); ok {
				if ws, ok := ss["wsSettings"].(map[string]interface{}); ok {
					if p, ok := ws["path"].(string); ok && path == "" {
						path = p
					}
					if hdrs, ok := ws["headers"].(map[string]interface{}); ok {
						headers = hdrs
						if hStr, ok := hdrs["Host"].(string); ok && hostHeader == "" {
							hostHeader = hStr
						}
					}
					if hStr, ok := ws["host"].(string); ok && hostHeader == "" {
						hostHeader = hStr
					}
				}
			}
		}

		if path != "" {
			wsObj["path"] = path
		}
		if hostHeader != "" {
			if headers == nil {
				headers = map[string]interface{}{}
			}
			headers["Host"] = hostHeader
		}
		if len(headers) > 0 {
			wsObj["headers"] = headers
		}
		streamSettings["wsSettings"] = wsObj

	} else if network == "xhttp" {
		xhttpObj := map[string]interface{}{}
		path := h.Path
		hostHeader := h.Host
		mode := "auto"
		var extra interface{}
		var headers map[string]interface{}

		if rawInbMap != nil {
			if ss, ok := rawInbMap["streamSettings"].(map[string]interface{}); ok {
				if xs, ok := ss["xhttpSettings"].(map[string]interface{}); ok {
					if p, ok := xs["path"].(string); ok && path == "" {
						path = p
					}
					if hStr, ok := xs["host"].(string); ok && hostHeader == "" {
						hostHeader = hStr
					}
						if hdrs, ok := xs["headers"].(map[string]interface{}); ok {
							headers = hdrs
							if hStr, ok := hdrs["Host"].(string); ok && hostHeader == "" {
								hostHeader = hStr
							}
						}

					if m, ok := xs["mode"].(string); ok && m != "" {
						mode = m
					}
					if ex, ok := xs["extra"]; ok {
						extra = ex
					}
				}
			}
		}
		if h.XhttpExtraParams != "" && h.XhttpExtraParams != "{}" && h.XhttpExtraParams != "null" {
			var hostExtra interface{}
			if err := json.Unmarshal([]byte(h.XhttpExtraParams), &hostExtra); err == nil {
				extra = hostExtra
			}
		}

		if mode != "" {
			xhttpObj["mode"] = mode
		}
		if path != "" {
			xhttpObj["path"] = path
		}
		if hostHeader != "" {
			xhttpObj["host"] = hostHeader
		}
		if extra != nil {
			xhttpObj["extra"] = extra
		}
		if headers != nil && len(headers) > 0 {
			xhttpObj["headers"] = headers
		}

		streamSettings["xhttpSettings"] = xhttpObj

	} else if network == "httpupgrade" {
		huObj := map[string]interface{}{}
		path := h.Path
		hostHeader := h.Host
		var headers map[string]interface{}
		if rawInbMap != nil {
			if ss, ok := rawInbMap["streamSettings"].(map[string]interface{}); ok {
				if hu, ok := ss["httpupgradeSettings"].(map[string]interface{}); ok {
					if p, ok := hu["path"].(string); ok && path == "" {
						path = p
					}
					if hdrs, ok := hu["headers"].(map[string]interface{}); ok {
						headers = hdrs
						if hStr, ok := hdrs["Host"].(string); ok && hostHeader == "" {
							hostHeader = hStr
						}
					}
					if hStr, ok := hu["host"].(string); ok && hostHeader == "" {
						hostHeader = hStr
					}
				}
			}
		}
		if path != "" {
			huObj["path"] = path
		}
		if hostHeader != "" {
			if headers == nil {
				headers = map[string]interface{}{}
			}
			headers["Host"] = hostHeader
		}
		if len(headers) > 0 {
			huObj["headers"] = headers
		}
		streamSettings["httpupgradeSettings"] = huObj

	} else if network == "grpc" {
		grpcObj := map[string]interface{}{}
		serviceName := h.Path
		authority := h.Host
		multiMode := false
		if rawInbMap != nil {
			if ss, ok := rawInbMap["streamSettings"].(map[string]interface{}); ok {
				if gs, ok := ss["grpcSettings"].(map[string]interface{}); ok {
					if sn, ok := gs["serviceName"].(string); ok && serviceName == "" {
						serviceName = sn
					}
					if auth, ok := gs["authority"].(string); ok && authority == "" {
						authority = auth
					}
					if mm, ok := gs["multiMode"].(bool); ok {
						multiMode = mm
					}
				}
			}
		}
		if serviceName != "" {
			grpcObj["serviceName"] = serviceName
		}
		if authority != "" {
			grpcObj["authority"] = authority
		}
		if multiMode {
			grpcObj["mode"] = true
		}
		streamSettings["grpcSettings"] = grpcObj
	}

	// Client sockopt: comes ONLY from h.SockoptParams (server inbound sockopt like trustedXForwardedFor is NOT copied)
	if h.SockoptParams != "" && h.SockoptParams != "{}" && h.SockoptParams != "null" {
		var sockoptMap map[string]interface{}
		if err := json.Unmarshal([]byte(h.SockoptParams), &sockoptMap); err == nil && len(sockoptMap) > 0 {
			streamSettings["sockopt"] = sockoptMap
		}
	}

	return map[string]interface{}{
		"tag":            tag,
		"protocol":       protocol,
		"settings":       settings,
		"streamSettings": streamSettings,
	}
}
