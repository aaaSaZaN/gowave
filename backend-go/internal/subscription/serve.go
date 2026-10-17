package subscription

import (
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"gowave/internal/database"
	"gowave/internal/httpx"
	"gowave/internal/subtpl"
	"gowave/internal/subtpl/ojson"
)

// srrContext is ISRRContext.
type srrContext struct {
	UserAgent                  string
	Hwid                       *hwidHeaders
	IsExtendedClient           bool
	ResponseType               string
	RuleName                   *string
	IP                         string
	Settings                   *subSettings
	OverrideTemplateName       string
	IgnoreHostXrayJSONTemplate bool
	HeadersToApply             *ojson.Object
	IgnoreServeJSON            bool
	DisableHwidCheck           bool
	Encryption                 *SRREncryption
	ExcludeHostsByTags         map[string]bool
	RespondWithRemarks         []string
}

// Node drops repeated values of these headers and joins the rest with ", "
// (cookie with "; ").
var nodeSingletonHeaders = map[string]bool{
	"age": true, "authorization": true, "content-length": true, "content-type": true, "etag": true, "expires": true,
	"from": true, "host": true, "if-modified-since": true, "if-unmodified-since": true, "last-modified": true,
	"location": true, "max-forwards": true, "proxy-authorization": true, "referer": true, "retry-after": true,
	"server": true, "user-agent": true,
}

// nodeHeaders rebuilds Express' req.headers for rule matching.
func nodeHeaders(r *http.Request) map[string]string {
	headers := map[string]string{}
	for name, values := range r.Header {
		key := strings.ToLower(name)
		switch {
		case nodeSingletonHeaders[key]:
			headers[key] = values[0]
		case key == "cookie":
			headers[key] = strings.Join(values, "; ")
		case key == "set-cookie":
			headers[key] = strings.Join(values, ",")
		default:
			headers[key] = strings.Join(values, ", ")
		}
	}
	if r.Host != "" {
		headers["host"] = r.Host
	}
	return headers
}

var (
	extendedClientPatterns = []*regexp.Regexp{
		regexp.MustCompile(`^FlClash ?X/`), regexp.MustCompile(`^Flowvy/`), regexp.MustCompile(`^prizrak-box/`),
		regexp.MustCompile(`^koala-clash/`), regexp.MustCompile(`^Happ/`), regexp.MustCompile(`^INCY/`),
	}
	jsonFallbackPatterns = []*regexp.Regexp{
		regexp.MustCompile(`^[Ss]treisand`), regexp.MustCompile(`^Happ/`), regexp.MustCompile(`^INCY/`),
		regexp.MustCompile(`^ktor-client`), regexp.MustCompile(`^V2Box`), regexp.MustCompile(`^io\.github\.saeeddev94\.xray/`),
		regexp.MustCompile(`^v2rayNG/(\d+\.\d+\.\d+)`), regexp.MustCompile(`^v2rayN/(\d+\.\d+\.\d+)`), regexp.MustCompile(`^v2plus/(\d+\.\d+\.\d+)`),
	}
	ruleRegexCache sync.Map
)

func matchesAny(patterns []*regexp.Regexp, userAgent string) bool {
	for _, pattern := range patterns {
		if pattern.MatchString(userAgent) {
			return true
		}
	}
	return false
}

// resolveExtendedClients ports ResponseRulesMiddleware.resolveExtendedClients.
func resolveExtendedClients(userAgent string, extra []string) (bool, error) {
	if matchesAny(extendedClientPatterns, userAgent) {
		return true, nil
	}
	for _, pattern := range extra {
		compiled, ok := ruleRegexCache.Load(pattern)
		if !ok {
			re, err := regexp.Compile(pattern)
			if err != nil {
				return false, err
			}
			compiled, _ = ruleRegexCache.LoadOrStore(pattern, re)
		}
		if compiled.(*regexp.Regexp).MatchString(userAgent) {
			return true, nil
		}
	}
	return false, nil
}

// responseRules ports ResponseRulesMiddleware. It writes the response and
// returns nil when the request ends here.
func (h *Handler) responseRules(w http.ResponseWriter, r *http.Request) *srrContext {
	userAgent := truncateHeader(r, "User-Agent")
	settings, err := h.loadSettings()
	if err != nil || settings.ResponseRules == nil || userAgent == nil || *userAgent == "" {
		nestError(w, r, http.StatusForbidden, "Forbidden")
		return nil
	}
	headers := nodeHeaders(r)
	headers["x-remnawave-injected-short-uuid"] = chi.URLParam(r, "shortUuid")
	result := settings.ResponseRules.Match(headers, chi.URLParam(r, "clientType"))
	if !result.Matched || result.ResponseType == "" {
		nestError(w, r, http.StatusForbidden, "Forbidden")
		return nil
	}
	var extra []string
	var mods *SRRResponseModifications
	if result.MatchedRule != nil {
		mods = result.MatchedRule.ResponseModifications
	}
	if mods != nil {
		extra = mods.AdditionalExtendedClientsRegex
	}
	extended, err := resolveExtendedClients(*userAgent, extra)
	if err != nil {
		nestError(w, r, http.StatusInternalServerError, "Internal server error")
		return nil
	}
	ctx := &srrContext{
		UserAgent: *userAgent, Hwid: extractHwidHeaders(r), IsExtendedClient: extended,
		ResponseType: result.ResponseType, IP: httpx.ClientIP(r), Settings: settings,
	}
	if result.MatchedRule != nil {
		name := result.MatchedRule.Name
		ctx.RuleName = &name
	}
	if mods != nil {
		if mods.Headers != nil && !mods.ApplyHeadersToEnd {
			for _, header := range mods.Headers {
				w.Header().Set(header.Key, header.Value)
			}
		}
		if mods.Headers != nil && mods.ApplyHeadersToEnd {
			ctx.HeadersToApply = ojson.New()
			for _, header := range mods.Headers {
				ctx.HeadersToApply.Set(header.Key, header.Value)
			}
		}
		ctx.OverrideTemplateName = mods.SubscriptionTemplate
		ctx.IgnoreHostXrayJSONTemplate = mods.IgnoreHostXrayJSONTemplate
		ctx.IgnoreServeJSON = mods.IgnoreServeJSONAtBaseSubscription
		ctx.DisableHwidCheck = mods.DisableHwidCheck
		ctx.Encryption = mods.Encryption
		if mods.ExcludeHostsByTags != nil {
			ctx.ExcludeHostsByTags = map[string]bool{}
			for _, tag := range mods.ExcludeHostsByTags {
				ctx.ExcludeHostsByTags[tag] = true
			}
		}
		if len(mods.RespondWithRemarks) > 0 {
			ctx.RespondWithRemarks = mods.RespondWithRemarks
		}
	}
	switch ctx.ResponseType {
	case "BLOCK":
		nestError(w, r, http.StatusForbidden, "Forbidden")
		return nil
	case "STATUS_CODE_404":
		nestError(w, r, http.StatusNotFound, "Not Found")
		return nil
	case "STATUS_CODE_451":
		nestError(w, r, http.StatusUnavailableForLegalReasons, "Unavailable For Legal Reasons")
		return nil
	case "SOCKET_DROP":
		conn, _, err := http.NewResponseController(w).Hijack()
		if err != nil {
			panic(http.ErrAbortHandler)
		}
		if tcp, ok := conn.(*net.TCPConn); ok {
			_ = tcp.SetLinger(0)
		}
		_ = conn.Close()
		return nil
	}
	return ctx
}

// GetSubscription serves /api/sub/{shortUuid} and /api/sub/{shortUuid}/{clientType}.
func (h *Handler) GetSubscription(w http.ResponseWriter, r *http.Request) {
	ctx := h.responseRules(w, r)
	if ctx == nil {
		return
	}
	h.serve(w, r, ctx, chi.URLParam(r, "shortUuid"))
}

// subscriptionResult is SubscriptionWithConfigResponse.
type subscriptionResult struct {
	Headers     *ojson.Object
	Body        string
	ContentType string
}

func notFound(w http.ResponseWriter, r *http.Request) {
	expressJSON(w, r, http.StatusNotFound, ojson.New("isFound", false, "statusCode", float64(404), "message", "Resource not found"))
}

func (h *Handler) findUser(column, value string) (*database.User, error) {
	var user database.User
	if err := h.db.Preload("Traffic").Where(column+" = ?", value).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// serve ports SubscriptionController.getSubscription with
// SubscriptionService.getSubscriptionByShortUuid.
func (h *Handler) serve(w http.ResponseWriter, r *http.Request, ctx *srrContext, shortUUID string) {
	if ctx.ResponseType == "BROWSER" {
		user, err := h.findUser("short_uuid", shortUUID)
		if err != nil {
			notFound(w, r)
			return
		}
		info, err := h.subscriptionInfo(user, nil, false)
		if err != nil {
			notFound(w, r)
			return
		}
		expressJSON(w, r, http.StatusOK, info)
		return
	}
	user, err := h.findUser("short_uuid", shortUUID)
	if err != nil {
		notFound(w, r)
		return
	}
	if ctx.OverrideTemplateName == "" && user.ExternalSquadUUID != nil {
		templateType := ctx.ResponseType
		if templateType == "XRAY_BASE64" {
			templateType = "XRAY_JSON"
		}
		if name, ok := h.squadTemplateName(*user.ExternalSquadUUID, templateType); ok {
			ctx.OverrideTemplateName = name
		}
	}
	settings, overrides := h.applySquadOverrides(ctx.Settings, user.ExternalSquadUUID)
	ctx.Settings = settings
	tplUser := templateUser(user)

	render := func(hosts []*subtpl.Host, overrides *subtpl.HostOverrides, fallback *subtpl.FallbackOptions) (string, string) {
		return subtpl.Render(subtpl.RenderRequest{
			ResponseType: ctx.ResponseType, IsExtendedClient: ctx.IsExtendedClient,
			OverrideTemplateName: ctx.OverrideTemplateName, IgnoreHostXrayJSONTemplate: ctx.IgnoreHostXrayJSONTemplate,
			Resolve: subtpl.ResolveOptions{
				Settings: settings.template(), Hosts: hosts, User: tplUser, HostOverrides: overrides, Fallback: fallback,
				ExcludeHostsByTags: ctx.ExcludeHostsByTags, SubPublicDomain: h.subPublicDomain,
			},
		}, h.templates)
	}

	if ctx.RespondWithRemarks != nil {
		body, contentType := render([]*subtpl.Host{}, nil, &subtpl.FallbackOptions{RespondWithRemarks: ctx.RespondWithRemarks})
		h.finalize(w, r, ctx, subscriptionResult{Headers: h.profileHeaders(tplUser, settings, false), Body: body, ContentType: contentType})
		return
	}

	var checkup *hwidCheckup
	if settings.Hwid.Enabled && !ctx.DisableHwidCheck {
		result := h.checkHwidDeviceLimit(user, ctx.Hwid, settings.Hwid, ctx.IP)
		checkup = &result
		if !result.SubscriptionAllowed {
			response := subscriptionResult{Headers: h.profileHeaders(tplUser, settings, false), Body: "", ContentType: "text/plain"}
			if !result.LimitBypassed {
				response.Headers.Set("x-hwid-active", "true")
			}
			if result.MaxDeviceReached && settings.Hwid.MaxDevicesAnnounce != nil && *settings.Hwid.MaxDevicesAnnounce != "" {
				response.Headers.Set("announce", subtpl.ReplaceTemplate("rwEncodeBase64:"+*settings.Hwid.MaxDevicesAnnounce,
					subtpl.TemplateValues(tplUser, settings.template(), h.subPublicDomain)))
			}
			if (result.MaxDeviceReached || result.HwidNotSupported) && settings.IsShowCustomRemarks {
				response.Body, response.ContentType = render([]*subtpl.Host{}, nil, &subtpl.FallbackOptions{
					ShowHwidMaxDeviceRemarks: result.MaxDeviceReached, ShowHwidNotSupportedRemarks: result.HwidNotSupported,
				})
			}
			if result.HwidNotSupported {
				response.Headers.Set("x-hwid-not-supported", "true")
			}
			if result.MaxDeviceReached {
				response.Headers.Set("x-hwid-max-devices-reached", "true")
			}
			response.Headers.Set("x-hwid-limit", "true")
			h.finalize(w, r, ctx, response)
			return
		}
	} else {
		h.upsertHwidQuietly(user, ctx.Hwid, ctx.IP)
	}

	if settings.ServeJSONAtBaseSubscription && ctx.ResponseType == "XRAY_BASE64" && !ctx.IgnoreServeJSON &&
		matchesAny(jsonFallbackPatterns, ctx.UserAgent) {
		ctx.ResponseType = "XRAY_JSON"
	}
	hosts, err := h.hostsForUser(user.ID, false, ctx.ResponseType == "XRAY_JSON" || ctx.ResponseType == "MIHOMO")
	if err != nil {
		notFound(w, r)
		return
	}
	h.recordRequest(user.ID, ctx.UserAgent, ctx.IP, ctx.RuleName, ctx.ResponseType)
	if settings.RandomizeHosts {
		hosts = subtpl.Shuffle(hosts)
	}
	body, contentType := render(hosts, overrides, nil)
	h.finalize(w, r, ctx, subscriptionResult{
		Headers: h.profileHeaders(tplUser, settings, checkup != nil && !checkup.LimitBypassed),
		Body:    body, ContentType: contentType,
	})
}

// finalize ports SubscriptionController.finalizeSubscriptionResponse.
func (h *Handler) finalize(w http.ResponseWriter, r *http.Request, ctx *srrContext, result subscriptionResult) {
	body, contentType := result.Body, result.ContentType
	if ctx.Encryption != nil {
		body = encryptResponseBody(body, ctx.Encryption)
		contentType = "text/plain"
	}
	for _, headers := range []*ojson.Object{result.Headers, ctx.HeadersToApply} {
		if headers == nil {
			continue
		}
		for _, key := range headers.Keys() {
			if value, ok := headers.Value(key).(string); ok {
				w.Header().Set(key, value)
			}
		}
	}
	expressSend(w, r, http.StatusOK, contentType, body)
}

// profileHeaders ports SubscriptionService.getUserProfileHeadersInfo.
func (h *Handler) profileHeaders(user *subtpl.User, settings *subSettings, hwidLimit bool) *ojson.Object {
	expire := int64(0)
	if user.ExpireAt.UTC().Year() != 2099 {
		expire = user.ExpireAt.Unix()
	}
	headers := ojson.New(
		"content-disposition", "attachment; filename="+user.Username,
		"subscription-userinfo", "upload=0; download="+strconv.FormatInt(user.UsedTrafficBytes, 10)+
			"; total="+strconv.FormatInt(user.TrafficLimitBytes, 10)+"; expire="+strconv.FormatInt(expire, 10),
	)
	if next := subtpl.NextTrafficResetAt(user.TrafficLimitStrategy, user.CreatedAt); next != nil {
		headers.Set("subscription-refill-date", strconv.FormatInt(next.Unix(), 10))
	}
	if hwidLimit {
		headers.Set("x-hwid-active", "true")
	}
	if settings.CustomResponseHeaders != nil {
		values := subtpl.TemplateValues(user, settings.template(), h.subPublicDomain)
		for _, key := range settings.CustomResponseHeaders.Keys() {
			if value, ok := settings.CustomResponseHeaders.Value(key).(string); ok {
				headers.Set(key, subtpl.ReplaceTemplate(value, values))
			}
		}
	}
	return headers
}

// recordRequest ports the ADD_SUBSCRIPTION_REQUEST_RECORD job: one history
// row, keeping the user's latest 24 (see historyWriter).
func (h *Handler) recordRequest(userID uint64, userAgent, ip string, ruleName *string, responseType string) {
	if h.disableHistory {
		return
	}
	h.history.add(database.UserSubscriptionRequestHistory{UserID: userID, RequestIP: optionalIP(ip), UserAgent: &userAgent,
		SrrRuleName: ruleName, SrrResponseType: responseType, RequestAt: time.Now().UTC()})
}
