package subscription

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"gowave/internal/database"
	"gowave/internal/httpx"
	"gowave/internal/subtpl"
	"gowave/internal/subtpl/ojson"
	"gowave/internal/users"
)

// protectedUser resolves the user named by the route of the protected
// subscriptions controller.
func (h *Handler) protectedUser(r *http.Request) (*database.User, error) {
	service := users.NewService(h.db)
	if id := chi.URLParam(r, "userId"); id != "" {
		n, err := strconv.ParseUint(id, 10, 64)
		if err != nil {
			return nil, err
		}
		return service.GetByID(n)
	}
	if name := chi.URLParam(r, "username"); name != "" {
		return service.GetByUsername(name)
	}
	return service.GetByShortUUID(chi.URLParam(r, "shortUuid"))
}

// InfoProtected serves GetSubscriptionById/ByUsername/ByShortUuidProtected.
func (h *Handler) InfoProtected(w http.ResponseWriter, r *http.Request) {
	user, err := h.protectedUser(r)
	if err != nil {
		httpx.Error(w, r, http.StatusNotFound, "A025", "User not found")
		return
	}
	info, err := h.subscriptionInfo(user, nil, true)
	if err != nil {
		httpx.Error(w, r, http.StatusInternalServerError, "A001", "Server error")
		return
	}
	httpx.Response(w, info)
}

// ListProtected serves GetSubscriptionsCommand.
func (h *Handler) ListProtected(w http.ResponseWriter, r *http.Request) {
	start, _ := strconv.Atoi(r.URL.Query().Get("start"))
	size, _ := strconv.Atoi(r.URL.Query().Get("size"))
	if size == 0 {
		size = 25
	}
	if start < 0 || size < 1 || size > 500 {
		httpx.Error(w, r, http.StatusBadRequest, "E000", "invalid pagination")
		return
	}
	records, total, err := users.NewService(h.db).GetAll(users.GetUsersQuery{Start: start, Size: size})
	if err != nil {
		httpx.Error(w, r, http.StatusInternalServerError, "A104", "Getting all subscriptions error")
		return
	}
	settings, err := h.loadSettings()
	if err != nil {
		httpx.Error(w, r, http.StatusInternalServerError, "A104", "Getting all subscriptions error")
		return
	}
	subscriptions := []any{}
	for _, user := range records {
		if info, err := h.subscriptionInfo(user, settings, true); err == nil {
			subscriptions = append(subscriptions, info)
		}
	}
	httpx.Response(w, ojson.New("total", float64(total), "subscriptions", subscriptions))
}

// RawProtected ports SubscriptionService.getRawSubscriptionByShortUuid.
func (h *Handler) RawProtected(w http.ResponseWriter, r *http.Request) {
	user, err := users.NewService(h.db).GetByShortUUID(chi.URLParam(r, "shortUuid"))
	if err != nil {
		httpx.Error(w, r, http.StatusNotFound, "A025", "User not found")
		return
	}
	withDisabledHosts := r.URL.Query().Get("withDisabledHosts") == "true"
	userAgent := "Unknown"
	if value := truncateHeader(r, "User-Agent"); value != nil {
		userAgent = *value
	}
	ip := httpx.ClientIP(r)
	hwid := extractHwidHeaders(r)
	baseSettings, err := h.loadSettings()
	if err != nil {
		httpx.Error(w, r, http.StatusNotFound, "A071", "Subscription settings not found")
		return
	}
	settings, overrides := h.applySquadOverrides(baseSettings, user.ExternalSquadUUID)
	tplUser := templateUser(user)
	headers := h.profileHeaders(tplUser, settings, false)

	var checkup *hwidCheckup
	if settings.Hwid.Enabled {
		result := h.checkHwidDeviceLimit(user, hwid, settings.Hwid, ip)
		checkup = &result
		if !result.LimitBypassed {
			headers.Set("x-hwid-active", "true")
		}
		if !result.SubscriptionAllowed {
			if result.MaxDeviceReached && settings.Hwid.MaxDevicesAnnounce != nil && *settings.Hwid.MaxDevicesAnnounce != "" {
				headers.Set("announce", subtpl.ReplaceTemplate("rwEncodeBase64:"+*settings.Hwid.MaxDevicesAnnounce,
					subtpl.TemplateValues(tplUser, settings.template(), h.subPublicDomain)))
			}
			if result.HwidNotSupported {
				headers.Set("x-hwid-not-supported", "true")
			}
			if result.MaxDeviceReached {
				headers.Set("x-hwid-max-devices-reached", "true")
			}
			headers.Set("x-hwid-limit", "true")
		}
	} else {
		h.upsertHwidQuietly(user, hwid, ip)
	}

	resolve := func(hosts []*subtpl.Host, fallback *subtpl.FallbackOptions) []*subtpl.Resolved {
		return subtpl.ResolveProxyConfig(subtpl.ResolveOptions{
			Settings: settings.template(), Hosts: hosts, User: tplUser, HostOverrides: overrides,
			Fallback: fallback, SubPublicDomain: h.subPublicDomain,
		})
	}
	var resolved []*subtpl.Resolved
	if checkup == nil || checkup.SubscriptionAllowed {
		hosts, err := h.hostsForUser(user.ID, withDisabledHosts, true)
		if err != nil {
			httpx.Error(w, r, http.StatusInternalServerError, "A050", "Get all hosts error")
			return
		}
		if settings.RandomizeHosts {
			hosts = subtpl.Shuffle(hosts)
		}
		resolved = resolve(hosts, nil)
	} else if settings.IsShowCustomRemarks && (checkup.MaxDeviceReached || checkup.HwidNotSupported) {
		resolved = resolve([]*subtpl.Host{}, &subtpl.FallbackOptions{
			ShowHwidMaxDeviceRemarks: checkup.MaxDeviceReached, ShowHwidNotSupportedRemarks: checkup.HwidNotSupported,
		})
	}
	h.recordRequest(user.ID, userAgent, ip, ptr("RAW"), "RAW")

	configs := make([]any, 0, len(resolved))
	for _, config := range resolved {
		configs = append(configs, config.Object())
	}
	var hwidValue any
	if checkup != nil {
		hwidValue = ojson.New("subscriptionAllowed", checkup.SubscriptionAllowed, "maxDeviceReached", checkup.MaxDeviceReached,
			"hwidNotSupported", checkup.HwidNotSupported, "limitBypassed", checkup.LimitBypassed)
	}
	httpx.Response(w, ojson.New(
		"user", ojson.FromGo(users.FormatUser(user, h.subPublicDomain)),
		"convertedUserInfo", ojson.New(
			"daysLeft", daysLeft(tplUser.ExpireAt),
			"trafficUsed", prettyBytes(tplUser.UsedTrafficBytes),
			"trafficLimit", prettyBytes(tplUser.TrafficLimitBytes),
			"lifetimeTrafficUsed", prettyBytes(tplUser.LifetimeUsedTrafficBytes),
			"hwidCheckup", hwidValue,
		),
		"headers", headers,
		"resolvedProxyConfigs", configs,
	))
}

func ptr[T any](value T) *T { return &value }

// ConnectionKeys ports SubscriptionService.getConnectionKeysByUserId.
func (h *Handler) ConnectionKeys(w http.ResponseWriter, r *http.Request) {
	user, err := h.protectedUser(r)
	if err != nil {
		httpx.Error(w, r, http.StatusNotFound, "A025", "User not found")
		return
	}
	settings, err := h.loadSettings()
	if err != nil {
		httpx.Error(w, r, http.StatusInternalServerError, "A001", "Server error")
		return
	}
	var overrides *subtpl.HostOverrides
	if user.ExternalSquadUUID != nil {
		settings, overrides = h.applySquadOverrides(settings, user.ExternalSquadUUID)
	}
	all, err := h.hostsForUser(user.ID, true, true)
	if err != nil {
		all = nil
	}
	var enabled, disabled, hidden []*subtpl.Host
	for _, host := range all {
		switch {
		case !host.IsDisabled && !host.IsHidden:
			enabled = append(enabled, host)
		case host.IsDisabled && !host.IsHidden:
			disabled = append(disabled, host)
		case host.IsHidden && !host.IsDisabled:
			hidden = append(hidden, host)
		}
	}
	tplUser := templateUser(user)
	keys := func(hosts []*subtpl.Host, allowEmpty bool) []any {
		if len(hosts) == 0 && !allowEmpty {
			return []any{}
		}
		resolved := subtpl.ResolveProxyConfig(subtpl.ResolveOptions{
			Settings: settings.template(), Hosts: hosts, User: tplUser, HostOverrides: overrides, SubPublicDomain: h.subPublicDomain,
		})
		return stringsToAny(subtpl.LinkList(resolved, false))
	}
	httpx.Response(w, ojson.New(
		"enabledKeys", keys(enabled, true),
		"disabledKeys", keys(disabled, false),
		"hiddenKeys", keys(hidden, false),
	))
}

// SubpageConfig ports SubscriptionService.getSubpageConfigByShortUuid; the
// rules see body.requestHeaders as given.
func (h *Handler) SubpageConfig(w http.ResponseWriter, r *http.Request) {
	var body struct {
		RequestHeaders map[string]string `json:"requestHeaders"`
	}
	if r.Body != nil {
		_ = json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20)).Decode(&body)
	}
	headers := body.RequestHeaders
	if headers == nil {
		headers = nodeHeaders(r)
	}
	var config *string
	var configs []*string
	if err := h.db.Table("users").Select("external_squads.subpage_config_uuid").
		Joins("JOIN external_squads ON external_squads.uuid = users.external_squad_uuid").
		Where("users.short_uuid = ? AND users.external_squad_uuid IS NOT NULL", chi.URLParam(r, "shortUuid")).
		Limit(1).Pluck("external_squads.subpage_config_uuid", &configs).Error; err == nil && len(configs) > 0 && configs[0] != nil && *configs[0] != "" {
		config = configs[0]
	}
	webpageAllowed := false
	if settings, err := h.loadSettings(); err == nil && settings.ResponseRules != nil {
		result := settings.ResponseRules.Match(headers, "")
		webpageAllowed = result.Matched && result.ResponseType == "BROWSER"
	}
	httpx.Response(w, ojson.New("subpageConfigUuid", optionalString(config), "webpageAllowed", webpageAllowed))
}

func optionalString(value *string) any {
	if value == nil {
		return nil
	}
	return *value
}

func formatInt(value int64) string {
	return strconv.FormatInt(value, 10)
}
