package subscription

import (
	"encoding/json"
	"strings"

	"remnawave-go/internal/database"
)

func GenerateSingboxJSON(user *database.User, hosts []XrayHostMeta, templateJSON string) (string, error) {
	var proxyOutbounds []map[string]interface{}
	var proxyTags []string

	for _, hm := range hosts {
		h := hm.Host
		if h.IsDisabled || h.IsHidden {
			continue
		}
		if IsExcluded(h.ExcludeFromSubscriptionTypes, "SINGBOX") {
			continue
		}

		ob := buildSingboxOutbound(user, hm)
		if ob != nil {
			proxyOutbounds = append(proxyOutbounds, ob)
			proxyTags = append(proxyTags, h.Remark)
		}
	}

	var base map[string]interface{}
	if templateJSON != "" && templateJSON != "{}" && templateJSON != "null" {
		_ = json.Unmarshal([]byte(templateJSON), &base)
	}
	if base == nil {
		base = map[string]interface{}{
			"outbounds": []interface{}{
				map[string]interface{}{
					"tag":       "→ Remnawave",
					"type":      "selector",
					"outbounds": nil,
				},
				map[string]interface{}{
					"tag":  "direct",
					"type": "direct",
				},
			},
		}
	}

	// Process outbounds in template
	rawOutbounds, _ := base["outbounds"].([]interface{})
	var newOutbounds []interface{}

	for _, ro := range rawOutbounds {
		oMap, ok := ro.(map[string]interface{})
		if !ok {
			newOutbounds = append(newOutbounds, ro)
			continue
		}

		oType, _ := oMap["type"].(string)
		if oType == "selector" || oType == "urltest" {
			existingList, _ := oMap["outbounds"].([]interface{})
			if len(existingList) == 0 {
				var tagsList []interface{}
				for _, tag := range proxyTags {
					tagsList = append(tagsList, tag)
				}
				oMap["outbounds"] = tagsList
			}
		}
		newOutbounds = append(newOutbounds, oMap)
	}

	for _, po := range proxyOutbounds {
		newOutbounds = append(newOutbounds, po)
	}

	base["outbounds"] = newOutbounds

	resBytes, err := json.MarshalIndent(base, "", "  ")
	if err != nil {
		return "{}", err
	}
	return string(resBytes), nil
}

func buildSingboxOutbound(user *database.User, hm XrayHostMeta) map[string]interface{} {
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

	// 1. Hysteria 2
	if protocol == "hysteria" {
		ob := map[string]interface{}{
			"type":        "hysteria2",
			"tag":         h.Remark,
			"server":      h.Address,
			"server_port": h.Port,
			"password":    user.VlessUUID,
			"tls": map[string]interface{}{
				"enabled":     true,
				"server_name": serverName,
				"alpn":        []string{"h3"},
			},
		}
		return ob
	}

	// 2. VLESS
	if protocol == "vless" {
		nodeUUID := user.VlessUUID
		if h.UUID == "00000000-0000-0000-0000-000000000000" {
			nodeUUID = "00000000-0000-0000-0000-000000000000"
		}
		ob := map[string]interface{}{
			"type":        "vless",
			"tag":         h.Remark,
			"server":      h.Address,
			"server_port": h.Port,
			"uuid":        nodeUUID,
		}

		flow := ResolveVlessFlow(rawInbMap, network, security)
		if flow == "xtls-rprx-vision" {
			ob["flow"] = "xtls-rprx-vision"
		}

		tlsObj := map[string]interface{}{}
		if security == "reality" {
			tlsObj["enabled"] = true
			tlsObj["server_name"] = serverName
			tlsObj["utls"] = map[string]interface{}{
				"enabled":     true,
				"fingerprint": fingerprint,
			}

			var realityCfg XrayRawRealitySettings
			if rawInb.StreamSettings != nil && rawInb.StreamSettings.RealitySettings != nil {
				realityCfg = *rawInb.StreamSettings.RealitySettings
			}
			if serverName == "" && len(realityCfg.ServerNames) > 0 {
				tlsObj["server_name"] = realityCfg.ServerNames[0]
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
			tlsObj["reality"] = map[string]interface{}{
				"enabled":    true,
				"public_key": publicKey,
				"short_id":   shortID,
			}
			ob["tls"] = tlsObj
		} else if security == "tls" {
			tlsObj["enabled"] = true
			tlsObj["server_name"] = serverName
			tlsObj["utls"] = map[string]interface{}{
				"enabled":     true,
				"fingerprint": fingerprint,
			}
			ob["tls"] = tlsObj
		}

		// Transport
		if network == "ws" {
			wsPath := h.Path
			wsHost := h.Host
			if wsPath == "" && rawInb.StreamSettings != nil && rawInb.StreamSettings.WsSettings != nil {
				if p, ok := rawInb.StreamSettings.WsSettings["path"].(string); ok {
					wsPath = p
				}
			}
			ob["transport"] = map[string]interface{}{
				"type": "ws",
				"path": wsPath,
				"headers": map[string]interface{}{
					"Host": wsHost,
				},
			}
		} else if network == "grpc" {
			sn := h.Path
			ob["transport"] = map[string]interface{}{
				"type":         "grpc",
				"service_name": sn,
			}
		}

		return ob
	}

	return nil
}
