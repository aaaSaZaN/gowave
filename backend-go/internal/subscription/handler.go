package subscription

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"

	"remnawave-go/internal/database"
)

type Handler struct {
	db              *gorm.DB
	generator       *Generator
	subPublicDomain string
}

func NewHandler(db *gorm.DB, subPublicDomain string) *Handler {
	return &Handler{
		db:              db,
		generator:       NewGenerator(),
		subPublicDomain: subPublicDomain,
	}
}

func (h *Handler) getHostsForUser(userID uint64) []database.Host {
	var user database.User
	if err := h.db.First(&user, userID).Error; err != nil {
		return nil
	}

	var count int64
	if err := h.db.Table("internal_squad_members").Where("user_id = ?", userID).Count(&count).Error; err != nil {
		log.Printf("[SUBSCRIPTION] failed to resolve squads for user id=%d: %v", userID, err)
		return nil
	}

	if count == 0 {
		return []database.Host{}
	}
	var hosts []database.Host

	type squadInbound struct {
		SquadUUID   string `gorm:"column:internal_squad_uuid"`
		InboundUUID string `gorm:"column:inbound_uuid"`
	}
	var squadInbounds []squadInbound
	if err := h.db.Table("internal_squad_inbounds").
		Select("internal_squad_inbounds.internal_squad_uuid, internal_squad_inbounds.inbound_uuid").
		Joins("JOIN internal_squad_members ON internal_squad_members.internal_squad_uuid = internal_squad_inbounds.internal_squad_uuid").
		Where("internal_squad_members.user_id = ?", userID).
		Scan(&squadInbounds).Error; err != nil {
		log.Printf("[SUBSCRIPTION] failed to resolve inbound access for user id=%d: %v", userID, err)
		return nil
	}

	squadsByInbound := make(map[string][]string)
	for _, link := range squadInbounds {
		squadsByInbound[link.InboundUUID] = append(squadsByInbound[link.InboundUUID], link.SquadUUID)
	}
	if len(squadsByInbound) == 0 {
		return []database.Host{}
	}

	if err := h.db.Where("is_disabled = ? AND is_hidden = ?", false, false).
		Order("view_position ASC").Find(&hosts).Error; err != nil {
		log.Printf("[SUBSCRIPTION] failed to load hosts for user id=%d: %v", userID, err)
		return nil
	}

	allSquadUUIDs := make([]string, 0)
	seenSquads := make(map[string]struct{})
	for _, squadUUIDs := range squadsByInbound {
		for _, squadUUID := range squadUUIDs {
			if _, exists := seenSquads[squadUUID]; exists {
				continue
			}
			seenSquads[squadUUID] = struct{}{}
			allSquadUUIDs = append(allSquadUUIDs, squadUUID)
		}
	}

	type hostSquadLink struct {
		HostUUID  string `gorm:"column:host_uuid"`
		SquadUUID string `gorm:"column:squad_uuid"`
	}
	var hostSquadLinks []hostSquadLink
	if err := h.db.Table("internal_squad_host_links").
		Select("host_uuid, squad_uuid").
		Where("squad_uuid IN ?", allSquadUUIDs).
		Scan(&hostSquadLinks).Error; err != nil {
		log.Printf("[SUBSCRIPTION] failed to resolve host squad links for user id=%d: %v", userID, err)
		return nil
	}
	hostSquads := make(map[string]map[string]struct{})
	for _, link := range hostSquadLinks {
		if hostSquads[link.HostUUID] == nil {
			hostSquads[link.HostUUID] = make(map[string]struct{})
		}
		hostSquads[link.HostUUID][link.SquadUUID] = struct{}{}
	}

	availableHosts := make([]database.Host, 0, len(hosts))
	for _, host := range hosts {
		inboundUUID := ""
		if host.ConfigProfileInboundUUID != nil {
			inboundUUID = *host.ConfigProfileInboundUUID
		}
		userSquads := squadsByInbound[inboundUUID]
		if len(userSquads) == 0 {
			continue
		}

		selectedSquads := hostSquads[host.UUID]

		allowOnly := strings.EqualFold(strings.TrimSpace(host.InternalSquadsMode), "ALLOW_ONLY")
		for _, squadUUID := range userSquads {
			_, isSelected := selectedSquads[squadUUID]
			if (allowOnly && isSelected) || (!allowOnly && !isSelected) {
				availableHosts = append(availableHosts, host)
				break
			}
		}
	}

	return availableHosts
}

type CustomRemarksConfig struct {
	ExpiredUsers           []string `json:"expiredUsers"`
	LimitedUsers           []string `json:"limitedUsers"`
	DisabledUsers          []string `json:"disabledUsers"`
	EmptyHosts             []string `json:"emptyHosts"`
	HWIDMaxDevicesExceeded []string `json:"HWIDMaxDevicesExceeded"`
	HWIDNotSupported       []string `json:"HWIDNotSupported"`
}

func (h *Handler) resolveFallbackRemarks(user *database.User, settings *database.SubscriptionSetting, hostCount int) []string {
	var customRemarks CustomRemarksConfig
	if settings.CustomRemarks != "" && settings.CustomRemarks != "null" && settings.CustomRemarks != "{}" {
		_ = json.Unmarshal([]byte(settings.CustomRemarks), &customRemarks)
	}

	if user.ExternalSquadUUID != nil && *user.ExternalSquadUUID != "" {
		var squad database.ExternalSquad
		if err := h.db.Where("uuid = ?", *user.ExternalSquadUUID).First(&squad).Error; err == nil {
			if squad.CustomRemarks != "" && squad.CustomRemarks != "null" && squad.CustomRemarks != "{}" {
				var sqRemarks CustomRemarksConfig
				if err := json.Unmarshal([]byte(squad.CustomRemarks), &sqRemarks); err == nil {
					if len(sqRemarks.EmptyHosts) > 0 {
						customRemarks.EmptyHosts = sqRemarks.EmptyHosts
					}
					if len(sqRemarks.ExpiredUsers) > 0 {
						customRemarks.ExpiredUsers = sqRemarks.ExpiredUsers
					}
					if len(sqRemarks.LimitedUsers) > 0 {
						customRemarks.LimitedUsers = sqRemarks.LimitedUsers
					}
					if len(sqRemarks.DisabledUsers) > 0 {
						customRemarks.DisabledUsers = sqRemarks.DisabledUsers
					}
					if len(sqRemarks.HWIDMaxDevicesExceeded) > 0 {
						customRemarks.HWIDMaxDevicesExceeded = sqRemarks.HWIDMaxDevicesExceeded
					}
					if len(sqRemarks.HWIDNotSupported) > 0 {
						customRemarks.HWIDNotSupported = sqRemarks.HWIDNotSupported
					}
				}
			}
		}
	}

	if settings.IsShowCustomRemarks && user.Status != "ACTIVE" {
		switch strings.ToUpper(user.Status) {
		case "EXPIRED":
			if len(customRemarks.ExpiredUsers) > 0 {
				return customRemarks.ExpiredUsers
			}
			return []string{"⌛ Subscription expired", "Contact support"}
		case "DISABLED":
			if len(customRemarks.DisabledUsers) > 0 {
				return customRemarks.DisabledUsers
			}
			return []string{"🚫 Subscription disabled", "Contact support"}
		case "LIMITED":
			if len(customRemarks.LimitedUsers) > 0 {
				return customRemarks.LimitedUsers
			}
			return []string{"🚧 Subscription limited", "Contact support"}
		}
	}

	if hostCount == 0 {
		if len(customRemarks.EmptyHosts) > 0 {
			return customRemarks.EmptyHosts
		}
		return []string{
			"→ Remnawave",
			"→ No hosts found",
			"→ Check Hosts tab",
			"→ Check Internal Squads tab",
		}
	}

	return nil
}

func (h *Handler) createFallbackHosts(user *database.User, remarks []string) ([]database.Host, []XrayHostMeta) {
	if len(remarks) == 0 {
		return nil, nil
	}

	hosts := make([]database.Host, 0, len(remarks))
	metas := make([]XrayHostMeta, 0, len(remarks))

	for _, remark := range remarks {
		finalRemark := strings.TrimSpace(h.formatHeaderValue(remark, user))
		if finalRemark == "" {
			continue
		}

		dummyHost := database.Host{
			UUID:          "00000000-0000-0000-0000-000000000000",
			Remark:        finalRemark,
			Address:       "0.0.0.0",
			Port:          1,
			SecurityLayer: "NONE",
			IsDisabled:    false,
			IsHidden:      false,
		}

		dummyInbound := database.ConfigProfileInbound{
			UUID:       "00000000-0000-0000-0000-000000000000",
			Type:       "vless",
			RawInbound: `{"protocol":"vless","settings":{"clients":[{"id":"00000000-0000-0000-0000-000000000000"}]},"streamSettings":{"network":"tcp","security":"none"}}`,
		}

		hosts = append(hosts, dummyHost)
		metas = append(metas, XrayHostMeta{
			Host:    &dummyHost,
			Inbound: &dummyInbound,
		})
	}

	return hosts, metas
}

func (h *Handler) getHostsWithInboundsForUser(userID uint64) []XrayHostMeta {
	hosts := h.getHostsForUser(userID)
	if len(hosts) == 0 {
		return nil
	}

	inboundUUIDs := make([]string, 0, len(hosts))
	for _, host := range hosts {
		if host.ConfigProfileInboundUUID != nil && *host.ConfigProfileInboundUUID != "" {
			inboundUUIDs = append(inboundUUIDs, *host.ConfigProfileInboundUUID)
		}
	}

	inboundMap := make(map[string]database.ConfigProfileInbound)
	if len(inboundUUIDs) > 0 {
		var inbounds []database.ConfigProfileInbound
		h.db.Where("uuid IN ?", inboundUUIDs).Find(&inbounds)
		for _, inb := range inbounds {
			inboundMap[inb.UUID] = inb
		}
	}

	result := make([]XrayHostMeta, 0, len(hosts))
	for i := range hosts {
		if hosts[i].ConfigProfileInboundUUID == nil {
			continue
		}
		if val, ok := inboundMap[*hosts[i].ConfigProfileInboundUUID]; ok {
			result = append(result, XrayHostMeta{
				Host:    &hosts[i],
				Inbound: &val,
			})
		}
	}
	return result
}

func (h *Handler) getTemplateForUser(user *database.User, templateType string, overrideTemplateName string) string {
	extractTmplContent := func(tmpl *database.SubscriptionTemplate) string {
		if templateType == "XRAY_JSON" || templateType == "SINGBOX" {
			return tmpl.TemplateJson
		}
		return tmpl.TemplateYaml
	}

	if overrideTemplateName != "" {
		var tmpl database.SubscriptionTemplate
		if err := h.db.Where("name = ? AND template_type = ?", overrideTemplateName, templateType).First(&tmpl).Error; err == nil {
			return extractTmplContent(&tmpl)
		}
	}

	if user.ExternalSquadUUID != nil && *user.ExternalSquadUUID != "" {
		var squadTmpl database.ExternalSquadTemplate
		if err := h.db.Where("external_squad_uuid = ? AND template_type = ?", *user.ExternalSquadUUID, templateType).First(&squadTmpl).Error; err == nil {
			var tmpl database.SubscriptionTemplate
			if err := h.db.Where("uuid = ?", squadTmpl.TemplateUUID).First(&tmpl).Error; err == nil {
				return extractTmplContent(&tmpl)
			}
		}
	}

	var tmpl database.SubscriptionTemplate
	if err := h.db.Where("template_type = ?", templateType).Order("view_position asc, created_at asc").First(&tmpl).Error; err == nil {
		return extractTmplContent(&tmpl)
	}

	return ""
}

func (h *Handler) getHostXrayTemplate(host *database.Host, defaultTmplJSON string) string {
	if host.XrayJsonTemplateUUID != nil && *host.XrayJsonTemplateUUID != "" {
		var tmpl database.SubscriptionTemplate
		if err := h.db.Where("uuid = ?", *host.XrayJsonTemplateUUID).First(&tmpl).Error; err == nil && tmpl.TemplateJson != "" && tmpl.TemplateJson != "{}" && tmpl.TemplateJson != "null" {
			return tmpl.TemplateJson
		}
	}
	return defaultTmplJSON
}

func formatHeaderValue(val string, user *database.User, subPublicDomain ...string) string {
	domain := ""
	if len(subPublicDomain) > 0 {
		domain = subPublicDomain[0]
	}
	subURL := user.ShortUUID
	if domain != "" {
		trimmed := strings.TrimRight(domain, "/")
		if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
			subURL = fmt.Sprintf("%s/%s", trimmed, user.ShortUUID)
		} else {
			subURL = fmt.Sprintf("https://%s/%s", trimmed, user.ShortUUID)
		}
	}

	val = strings.ReplaceAll(val, "{{USERNAME}}", user.Username)
	val = strings.ReplaceAll(val, "{{username}}", user.Username)
	val = strings.ReplaceAll(val, "{{user.username}}", user.Username)
	val = strings.ReplaceAll(val, "{{SHORT_UUID}}", user.ShortUUID)
	val = strings.ReplaceAll(val, "{{short_uuid}}", user.ShortUUID)
	val = strings.ReplaceAll(val, "{{user.shortUuid}}", user.ShortUUID)
	val = strings.ReplaceAll(val, "{{SUBSCRIPTION_URL}}", subURL)
	val = strings.ReplaceAll(val, "{{subscription_url}}", subURL)
	val = strings.ReplaceAll(val, "{{user.trafficLimit}}", fmt.Sprintf("%d", user.TrafficLimitBytes))
	val = strings.ReplaceAll(val, "{{TRAFFIC_LIMIT_BYTES}}", fmt.Sprintf("%d", user.TrafficLimitBytes))
	val = strings.ReplaceAll(val, "{{TOTAL_TRAFFIC_BYTES}}", fmt.Sprintf("%d", user.TrafficLimitBytes))

	if strings.HasPrefix(val, "rwEncodeBase64:") {
		raw := strings.TrimPrefix(val, "rwEncodeBase64:")
		return "base64:" + base64.StdEncoding.EncodeToString([]byte(raw))
	}
	return val
}

func (h *Handler) formatHeaderValue(val string, user *database.User) string {
	return formatHeaderValue(val, user, h.subPublicDomain)
}

func (h *Handler) GetSubscription(w http.ResponseWriter, r *http.Request) {
	shortUUID := chi.URLParam(r, "shortUuid")
	var user database.User
	if err := h.db.Preload("Traffic").Where("short_uuid = ?", shortUUID).First(&user).Error; err != nil {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"message":"Subscription not found"}`))
		return
	}

	// Load SubscriptionSetting
	var settings database.SubscriptionSetting
	h.db.First(&settings)

	if user.Status != "ACTIVE" && !settings.IsShowCustomRemarks {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"message":"Account is disabled"}`))
		return
	}

	// Prepare request headers for SRR
	reqHeaders := make(map[string]string)
	for k, v := range r.Header {
		if len(v) > 0 {
			reqHeaders[strings.ToLower(k)] = v[0]
		}
	}

	clientType := chi.URLParam(r, "clientType")
	reqHeaders["x-remnawave-injected-client-type"] = clientType
	reqHeaders["x-remnawave-injected-user-username"] = user.Username
	reqHeaders["x-remnawave-injected-short-uuid"] = user.ShortUUID

	// Match SRR
	srrRes := MatchSRRRules(settings.ResponseRules, reqHeaders, clientType)
	var matchedRuleName *string
	matchedResponseType := "UNKNOWN"

	if srrRes.Matched {
		matchedResponseType = srrRes.ResponseType
		if srrRes.MatchedRule != nil {
			matchedRuleName = &srrRes.MatchedRule.Name
		}
	}

	// Client IP extraction
	clientIP := r.Header.Get("X-Remnawave-Real-IP")
	if clientIP == "" {
		clientIP = r.Header.Get("x-remnawave-real-ip")
	}
	if clientIP == "" {
		clientIP = r.Header.Get("CF-Connecting-IP")
	}
	if clientIP == "" {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			clientIP = strings.TrimSpace(parts[0])
		}
	}
	if clientIP == "" {
		clientIP = r.Header.Get("X-Real-IP")
	}
	if clientIP == "" {
		clientIP = r.Header.Get("True-Client-IP")
	}
	if clientIP == "" {
		clientIP = r.RemoteAddr
		if colon := strings.LastIndex(clientIP, ":"); colon != -1 {
			clientIP = clientIP[:colon]
		}
	}
	clientIP = strings.Trim(clientIP, "[]")

	// Record request in history
	ua := r.Header.Get("User-Agent")
	go func(uID uint64, ip, userAgent string, ruleName *string, respType string) {
		h.db.Create(&database.UserSubscriptionRequestHistory{
			UserID:          uID,
			RequestIP:       &ip,
			UserAgent:       &userAgent,
			SrrRuleName:     ruleName,
			SrrResponseType: respType,
			RequestAt:       time.Now().UTC(),
		})
	}(user.ID, clientIP, ua, matchedRuleName, matchedResponseType)

	// HWID tracking
	hwid := r.Header.Get("X-HWID")
	if hwid == "" {
		hwid = r.Header.Get("x-hwid")
	}
	if len(hwid) >= 10 && len(hwid) <= 64 {
		var platform *string
		if p := r.Header.Get("X-Device-OS"); p != "" {
			platform = &p
		} else if p := r.Header.Get("x-device-os"); p != "" {
			platform = &p
		}
		var osVersion *string
		if v := r.Header.Get("X-Ver-OS"); v != "" {
			osVersion = &v
		} else if v := r.Header.Get("x-ver-os"); v != "" {
			osVersion = &v
		}
		var deviceModel *string
		if m := r.Header.Get("X-Device-Model"); m != "" {
			deviceModel = &m
		} else if m := r.Header.Get("x-device-model"); m != "" {
			deviceModel = &m
		}
		var reqIP *string
		if clientIP != "" {
			reqIP = &clientIP
		}
		var userAgent *string
		if ua != "" {
			userAgent = &ua
		}

		go func(hDev database.HwidDevice) {
			var existing database.HwidDevice
			if err := h.db.Where("hwid = ? AND user_id = ?", hDev.HWID, hDev.UserID).First(&existing).Error; err != nil {
				h.db.Create(&hDev)
			} else {
				h.db.Model(&existing).Updates(map[string]interface{}{
					"platform":     hDev.Platform,
					"os_version":   hDev.OSVersion,
					"device_model": hDev.DeviceModel,
					"user_agent":   hDev.UserAgent,
					"request_ip":   hDev.RequestIP,
					"updated_at":   hDev.UpdatedAt,
				})
			}
		}(database.HwidDevice{
			HWID:        hwid,
			UserID:      user.ID,
			Platform:    platform,
			OSVersion:   osVersion,
			DeviceModel: deviceModel,
			UserAgent:   userAgent,
			RequestIP:   reqIP,
			CreatedAt:   time.Now().UTC(),
			UpdatedAt:   time.Now().UTC(),
		})
	}

	// Handle response actions
	if matchedResponseType == "BLOCK" {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"message":"Subscription access blocked"}`))
		return
	} else if matchedResponseType == "STATUS_CODE_404" {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"message":"Not found"}`))
		return
	} else if matchedResponseType == "STATUS_CODE_451" {
		w.WriteHeader(http.StatusUnavailableForLegalReasons)
		w.Write([]byte(`{"message":"Unavailable for legal reasons"}`))
		return
	} else if matchedResponseType == "SOCKET_DROP" {
		if hj, ok := w.(http.Hijacker); ok {
			conn, _, _ := hj.Hijack()
			if conn != nil {
				conn.Close()
				return
			}
		}
		return
	} else if matchedResponseType == "BROWSER" {
		// Browser subscription redirects to public subscription page
		subURL := fmt.Sprintf("/%s", user.ShortUUID)
		http.Redirect(w, r, subURL, http.StatusFound)
		return
	}

	// Prepare Response Headers
	respHeaders := make(map[string]string)

	// 1. Global SubscriptionSetting Headers (BASE)
	if settings.CustomResponseHeaders != "" && settings.CustomResponseHeaders != "{}" {
		var globalHeaders map[string]string
		if err := json.Unmarshal([]byte(settings.CustomResponseHeaders), &globalHeaders); err == nil {
			for k, v := range globalHeaders {
				respHeaders[k] = h.formatHeaderValue(v, &user)
			}
		}
	}

	// 2. External Squad Headers (OVERRIDES BASE)
	if user.ExternalSquadUUID != nil && *user.ExternalSquadUUID != "" {
		var squad database.ExternalSquad
		if err := h.db.Where("uuid = ?", *user.ExternalSquadUUID).First(&squad).Error; err == nil {
			for _, key := range squad.ResponseHeadersRemove {
				delete(respHeaders, key)
			}
			if squad.ResponseHeadersAdd != "" && squad.ResponseHeadersAdd != "{}" && squad.ResponseHeadersAdd != "[]" {
				var squadAdd map[string]string
				if err := json.Unmarshal([]byte(squad.ResponseHeadersAdd), &squadAdd); err == nil {
					for k, v := range squadAdd {
						respHeaders[k] = h.formatHeaderValue(v, &user)
					}
				}
			}
		}
	}

	// 3. SRR Modifications
	overrideTmpl := ""
	if srrRes.Matched && srrRes.MatchedRule != nil && srrRes.MatchedRule.ResponseModifications != nil {
		mods := srrRes.MatchedRule.ResponseModifications
		overrideTmpl = mods.SubscriptionTemplate
		for _, hMod := range mods.Headers {
			respHeaders[hMod.Key] = h.formatHeaderValue(hMod.Value, &user)
		}
	}

	// User-info header standard (Clash / Stash / Sing-box / etc.)
	var usedBytes uint64
	if user.Traffic != nil {
		usedBytes = user.Traffic.UsedTrafficBytes
	}
	respHeaders["subscription-userinfo"] = fmt.Sprintf("upload=0; download=%d; total=%d; expire=%d",
		usedBytes, user.TrafficLimitBytes, user.ExpireAt.Unix())

	for k, v := range respHeaders {
		w.Header().Set(k, v)
	}

	var hosts []database.Host
	var hostsMeta []XrayHostMeta
	if user.Status == "ACTIVE" {
		hosts = h.getHostsForUser(user.ID)
		hostsMeta = h.getHostsWithInboundsForUser(user.ID)
	}

	fallbackRemarks := h.resolveFallbackRemarks(&user, &settings, len(hosts))
	if len(fallbackRemarks) > 0 {
		hosts, hostsMeta = h.createFallbackHosts(&user, fallbackRemarks)
	}

	// Determine output content
	var contentType string
	var body string

	targetType := matchedResponseType
	if targetType == "UNKNOWN" {
		uaLower := strings.ToLower(ua)
		if strings.Contains(uaLower, "happ") || strings.Contains(uaLower, "v2raytun") {
			targetType = "XRAY_JSON"
		} else if strings.Contains(uaLower, "clash") || strings.Contains(uaLower, "flclash") || strings.Contains(uaLower, "mihomo") {
			targetType = "MIHOMO"
		} else if strings.Contains(uaLower, "stash") {
			targetType = "STASH"
		} else if strings.Contains(uaLower, "sing-box") || strings.Contains(uaLower, "singbox") {
			targetType = "SINGBOX"
		} else {
			targetType = "XRAY_BASE64"
		}
	}

	if targetType == "XRAY_JSON" {
		contentType = "application/json; charset=utf-8"
		tmplJSON := h.getTemplateForUser(&user, "XRAY_JSON", overrideTmpl)
		jsonBody, err := GenerateXrayJSON(&user, hostsMeta, tmplJSON, func(host *database.Host) string {
			return h.getHostXrayTemplate(host, tmplJSON)
		})
		if err != nil {
			http.Error(w, `{"message":"Failed to generate xray json"}`, http.StatusInternalServerError)
			return
		}
		body = jsonBody
	} else if targetType == "MIHOMO" || targetType == "CLASH" || targetType == "STASH" {
		contentType = "text/yaml; charset=utf-8"
		tmplYAML := h.getTemplateForUser(&user, targetType, overrideTmpl)
		if tmplYAML == "" && targetType != "MIHOMO" {
			tmplYAML = h.getTemplateForUser(&user, "MIHOMO", overrideTmpl)
		}
		yamlBody, err := GenerateMihomoYAML(&user, hostsMeta, tmplYAML, targetType)
		if err != nil {
			http.Error(w, `{"message":"Failed to generate yaml config"}`, http.StatusInternalServerError)
			return
		}
		body = yamlBody
	} else if targetType == "SINGBOX" {
		contentType = "application/json; charset=utf-8"
		tmplJSON := h.getTemplateForUser(&user, "SINGBOX", overrideTmpl)
		jsonBody, err := GenerateSingboxJSON(&user, hostsMeta, tmplJSON)
		if err != nil {
			http.Error(w, `{"message":"Failed to generate singbox json"}`, http.StatusInternalServerError)
			return
		}
		body = jsonBody
	} else {
		contentType = "text/plain; charset=utf-8"
		body = h.generator.generateBase64Links(&user, hosts)
	}

	w.Header().Set("Content-Type", contentType)
	w.Write([]byte(body))
}

type SubscriptionInfoUser struct {
	ShortUUID                string    `json:"shortUuid"`
	DaysLeft                 int       `json:"daysLeft"`
	TrafficUsed              string    `json:"trafficUsed"`
	TrafficLimit             string    `json:"trafficLimit"`
	LifetimeTrafficUsed      string    `json:"lifetimeTrafficUsed"`
	TrafficUsedBytes         string    `json:"trafficUsedBytes"`
	TrafficLimitBytes        string    `json:"trafficLimitBytes"`
	LifetimeTrafficUsedBytes string    `json:"lifetimeTrafficUsedBytes"`
	Username                 string    `json:"username"`
	ExpiresAt                time.Time `json:"expiresAt"`
	IsActive                 bool      `json:"isActive"`
	UserStatus               string    `json:"userStatus"`
	TrafficLimitStrategy     string    `json:"trafficLimitStrategy"`
}

type SubscriptionInfoResponse struct {
	IsFound         bool                 `json:"isFound"`
	User            SubscriptionInfoUser `json:"user"`
	Links           []string             `json:"links"`
	SSConfLinks     map[string]string    `json:"ssConfLinks"`
	SubscriptionURL string               `json:"subscriptionUrl"`
}

type SubscriptionInfoWrapper struct {
	Response SubscriptionInfoResponse `json:"response"`
}

func (h *Handler) GetSubscriptionInfo(w http.ResponseWriter, r *http.Request) {
	shortUUID := chi.URLParam(r, "shortUuid")
	if shortUUID == "" {
		http.Error(w, `{"message":"Subscription short UUID is required"}`, http.StatusBadRequest)
		return
	}

	var user database.User
	if err := h.db.Preload("Traffic").Where("short_uuid = ?", shortUUID).First(&user).Error; err != nil {
		http.Error(w, `{"message":"User not found"}`, http.StatusNotFound)
		return
	}

	daysLeft := 0
	expiresAt := time.Now().AddDate(1, 0, 0)
	if !user.ExpireAt.IsZero() {
		expiresAt = user.ExpireAt
		daysLeft = int(time.Until(expiresAt).Hours() / 24)
		if daysLeft < 0 {
			daysLeft = 0
		}
	}

	var usedBytes uint64
	var lifetimeBytes uint64
	if user.Traffic != nil {
		usedBytes = user.Traffic.UsedTrafficBytes
		lifetimeBytes = user.Traffic.LifetimeUsedTrafficBytes
	}

	trafficLimitStr := "Unlimited"
	if user.TrafficLimitBytes > 0 {
		trafficLimitStr = formatBytes(int64(user.TrafficLimitBytes))
	}

	userStatus := user.Status
	if userStatus == "" {
		userStatus = "ACTIVE"
	}
	trafficLimitStrategy := user.TrafficLimitStrategy
	if trafficLimitStrategy == "" {
		trafficLimitStrategy = "NO_RESET"
	}

	subscriptionURL := ""
	if h.subPublicDomain != "" {
		trimmed := strings.TrimRight(h.subPublicDomain, "/")
		if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
			subscriptionURL = fmt.Sprintf("%s/%s", trimmed, user.ShortUUID)
		} else {
			subscriptionURL = fmt.Sprintf("https://%s/%s", trimmed, user.ShortUUID)
		}
	} else {
		subscriptionURL = fmt.Sprintf("https://%s/%s", r.Host, user.ShortUUID)
	}

	var settings database.SubscriptionSetting
	h.db.First(&settings)

	var hosts []database.Host
	if user.Status == "ACTIVE" {
		hosts = h.getHostsForUser(user.ID)
	}
	fallbackRemarks := h.resolveFallbackRemarks(&user, &settings, len(hosts))
	if len(fallbackRemarks) > 0 {
		hosts, _ = h.createFallbackHosts(&user, fallbackRemarks)
	}
	rawLinks := h.generator.generateRawLinks(&user, hosts)

	infoResp := SubscriptionInfoResponse{
		IsFound: true,
		User: SubscriptionInfoUser{
			ShortUUID:                user.ShortUUID,
			DaysLeft:                 daysLeft,
			TrafficUsed:              formatBytes(int64(usedBytes)),
			TrafficLimit:             trafficLimitStr,
			LifetimeTrafficUsed:      formatBytes(int64(lifetimeBytes)),
			TrafficUsedBytes:         strconv.FormatUint(usedBytes, 10),
			TrafficLimitBytes:        strconv.FormatUint(user.TrafficLimitBytes, 10),
			LifetimeTrafficUsedBytes: strconv.FormatUint(lifetimeBytes, 10),
			Username:                 user.Username,
			ExpiresAt:                expiresAt,
			IsActive:                 user.Status == "ACTIVE",
			UserStatus:               userStatus,
			TrafficLimitStrategy:     trafficLimitStrategy,
		},
		Links:           rawLinks,
		SSConfLinks:     map[string]string{},
		SubscriptionURL: subscriptionURL,
	}

	resp := SubscriptionInfoWrapper{
		Response: infoResp,
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(resp)
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
