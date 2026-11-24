package subscription

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"

	"gowave/internal/configprofiles"
	"gowave/internal/database"
	"gowave/internal/httpx"
	"gowave/internal/subtpl"
)

type hostPreviewRequest struct {
	HostUUID         string                          `json:"hostUuid"`
	Host             *configprofiles.CreateHostInput `json:"host"`
	UserAgent        string                          `json:"userAgent"`
	RequestHeaders   []previewHeader                 `json:"requestHeaders"`
	PreviewAsEnabled bool                            `json:"previewAsEnabled"`
}

type previewHeader struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type hostPreviewResponse struct {
	ClientType     string            `json:"clientType"`
	RuleName       *string           `json:"ruleName"`
	Status         *int              `json:"status"`
	ContentType    string            `json:"contentType"`
	Headers        map[string]string `json:"headers"`
	PreviewContent string            `json:"previewContent"`
}

var (
	previewHeaderName  = regexp.MustCompile("^[!#$%&'*+.^_`|~0-9A-Za-z-]+$")
	invalidHeaderValue = regexp.MustCompile(`[\x00-\x08\x0a-\x1f\x7f]`)
)

func (h *Handler) PreviewHost(w http.ResponseWriter, r *http.Request) {
	var input hostPreviewRequest
	if err := httpx.Decode(r, 1<<20, &input); err != nil || (input.Host == nil) == (input.HostUUID == "") {
		httpx.Error(w, r, http.StatusBadRequest, "GW400", "Provide either a saved host UUID or a draft host")
		return
	}

	const shortUUID = "preview"
	path := "/api/sub/" + shortUUID
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, path, nil)
	if err != nil {
		httpx.Error(w, r, http.StatusInternalServerError, httpx.CodeInternal, "Could not prepare the preview request")
		return
	}
	route := chi.NewRouteContext()
	route.URLParams.Add("shortUuid", shortUUID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, route))
	req.Host = h.subPublicDomain
	if invalidHeaderValue.MatchString(input.UserAgent) {
		httpx.Error(w, r, http.StatusBadRequest, "GW400", "Invalid User-Agent")
		return
	}
	req.Header.Set("User-Agent", input.UserAgent)
	for _, header := range input.RequestHeaders {
		name := http.CanonicalHeaderKey(header.Name)
		if name == "User-Agent" || !previewHeaderName.MatchString(name) || invalidHeaderValue.MatchString(header.Value) {
			httpx.Error(w, r, http.StatusBadRequest, "GW400", "Invalid request header. Set User-Agent in its own field.")
			return
		}
		if name == "Host" {
			req.Host = header.Value
		} else {
			req.Header.Add(name, header.Value)
		}
	}

	if input.Host != nil && (strings.TrimSpace(input.Host.Address) == "" || strings.TrimSpace(input.Host.Remark) == "") {
		httpx.Error(w, r, http.StatusBadRequest, "GW400", "Enter a host address and remark before previewing")
		return
	}
	host, err := h.loadPreviewHost(r.Context(), input)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		httpx.Error(w, r, http.StatusNotFound, "GW404", "Host or inbound not found")
		return
	}
	if err != nil {
		httpx.Error(w, r, http.StatusInternalServerError, httpx.CodeInternal, "Could not load host for preview")
		return
	}
	if host.ConfigProfileInbound == nil {
		httpx.Error(w, r, http.StatusBadRequest, "GW400", "Select a configuration profile and inbound to preview this host")
		return
	}
	if input.PreviewAsEnabled {
		host.IsDisabled = false
	}

	// Only the subscription response is captured. The preview user has no database row.
	recorder := httptest.NewRecorder()
	rule := h.responseRules(recorder, req)
	result := hostPreviewResponse{Headers: make(map[string]string)}
	if rule != nil {
		result.RuleName = rule.RuleName
		if rule.ResponseType == "SOCKET_DROP" {
			result.ClientType = rule.ResponseType
			httpx.Response(w, result)
			return
		}

		user := &subtpl.User{
			ShortUUID: shortUUID, Username: "preview", Status: "ACTIVE", TrafficLimitStrategy: "NO_RESET",
			ExpireAt: time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC), CreatedAt: subtpl.Now(),
			VlessUUID:      "11111111-2222-4333-8444-555555555555",
			TrojanPassword: "preview-password", SsPassword: "preview-password",
		}
		if !writeRuleError(recorder, req, rule.ResponseType) {
			h.serveHostPreview(recorder, req, rule, user, host)
		}
		result.ClientType = rule.ResponseType
	}
	result.Status = &recorder.Code
	result.ContentType = recorder.Header().Get("Content-Type")
	result.PreviewContent = recorder.Body.String()
	for name := range recorder.Header() {
		result.Headers[name] = recorder.Header().Get(name)
	}
	httpx.Response(w, result)
}

func (h *Handler) loadPreviewHost(ctx context.Context, input hostPreviewRequest) (*database.Host, error) {
	db := h.db.WithContext(ctx)
	if input.Host == nil {
		var host database.Host
		err := db.Preload("ConfigProfileInbound").Preload("XrayTemplate").Where("uuid = ?", input.HostUUID).Take(&host).Error
		return &host, err
	}

	host := input.Host.BuildHost()
	if host.ConfigProfileInboundUUID != nil {
		var inbound database.ConfigProfileInbound
		query := db.Where("uuid = ?", *host.ConfigProfileInboundUUID)
		if host.ConfigProfileUUID != nil {
			query = query.Where("profile_uuid = ?", *host.ConfigProfileUUID)
		}
		if err := query.Take(&inbound).Error; err != nil {
			return nil, err
		}
		host.ConfigProfileInbound = &inbound
	}
	if host.XrayJsonTemplateUUID != nil {
		var template database.SubscriptionTemplate
		if err := db.Where("uuid = ?", *host.XrayJsonTemplateUUID).Take(&template).Error; err != nil {
			return nil, err
		}
		host.XrayTemplate = &template
	}
	return host, nil
}

func (h *Handler) serveHostPreview(w http.ResponseWriter, r *http.Request, ctx *srrContext, user *subtpl.User, host *database.Host) {
	settings := ctx.Settings
	if ctx.ResponseType != "BROWSER" {
		if h.serveRuleRemarks(w, r, ctx, user) {
			return
		}
		if settings.Hwid.Enabled && !ctx.DisableHwidCheck {
			// The sample subscriber starts with zero registered devices.
			if ctx.Hwid == nil {
				h.serveHwidRejection(w, r, ctx, user, hwidCheckup{HwidNotSupported: true})
				return
			}
			if settings.Hwid.FallbackDeviceLimit <= 0 {
				h.serveHwidRejection(w, r, ctx, user, checkupLimited)
				return
			}
		}
		ctx.resolveConfigType()
	}

	var hosts []*subtpl.Host
	if !host.IsDisabled && (!host.IsHidden || ctx.includeHiddenHosts()) {
		inbound := host.ConfigProfileInbound
		var xrayTemplate any
		if host.XrayTemplate != nil {
			xrayTemplate = parseJSONColumn(host.XrayTemplate.TemplateJson)
		}
		hosts = append(hosts, templateHost(host, parseJSONColumn(inbound.RawInbound), inbound.Tag, xrayTemplate))
	}

	if ctx.ResponseType == "BROWSER" {
		var links []string
		if !settings.Hwid.Enabled {
			links = h.subscriptionLinks(user, settings, hosts, nil)
		}
		expressJSON(w, r, http.StatusOK, h.browserInfo(user, links))
		return
	}

	body, contentType := h.renderConfig(ctx, user, hosts, nil, nil)
	h.finalize(w, r, ctx, subscriptionResult{
		Headers: h.profileHeaders(user, settings, settings.Hwid.Enabled && !ctx.DisableHwidCheck),
		Body:    body, ContentType: contentType,
	})
}
