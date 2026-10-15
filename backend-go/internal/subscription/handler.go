package subscription

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"

	"remnawave-go/internal/database"
	"remnawave-go/internal/httpx"
	"remnawave-go/internal/subtpl"
	"remnawave-go/internal/subtpl/js"
	"remnawave-go/internal/subtpl/ojson"
)

// Handler ports upstream's subscription module on top of internal/subtpl.
type Handler struct {
	db              *gorm.DB
	subPublicDomain string
	disableHistory  bool
}

func NewHandler(db *gorm.DB, subPublicDomain string) *Handler {
	return &Handler{db: db, subPublicDomain: subPublicDomain}
}

// SetDisableHistory follows SERVICE_DISABLE_SRH_RECORDS.
func (h *Handler) SetDisableHistory(disabled bool) {
	h.disableHistory = disabled
}

// GetSubscriptionInfo serves GetSubscriptionInfoByShortUuidCommand.
func (h *Handler) GetSubscriptionInfo(w http.ResponseWriter, r *http.Request) {
	user, err := h.findUser("short_uuid", chi.URLParam(r, "shortUuid"))
	if err != nil {
		httpx.Error(w, r, http.StatusNotFound, "A025", "User not found")
		return
	}
	info, err := h.subscriptionInfo(user, nil, false)
	if err != nil {
		httpx.Error(w, r, http.StatusInternalServerError, "A001", "Server error")
		return
	}
	httpx.Response(w, info)
}

// daysLeft is dayjs(expireAt).diff(dayjs(), 'day'): truncated toward zero.
func daysLeft(expireAt time.Time) float64 {
	return float64(int64(expireAt.Sub(subtpl.Now()) / (24 * time.Hour)))
}

func prettyBytes(value int64) string {
	return js.PrettyBytes(float64(value), false, 0)
}

// subscriptionInfo ports SubscriptionService.getSubscriptionInfo; settings
// may be preloaded by the caller.
func (h *Handler) subscriptionInfo(user *database.User, settings *subSettings, authenticated bool) (*ojson.Object, error) {
	if settings == nil {
		loaded, err := h.loadSettings()
		if err != nil {
			return nil, err
		}
		settings = loaded
	}
	var overrides *subtpl.HostOverrides
	if user.ExternalSquadUUID != nil {
		settings, overrides = h.applySquadOverrides(settings, user.ExternalSquadUUID)
	}
	tplUser := templateUser(user)
	links := []string{}
	if !settings.Hwid.Enabled || authenticated {
		hosts, err := h.hostsForUser(user.ID, false, false)
		if err != nil {
			hosts = nil
		}
		resolved := subtpl.ResolveProxyConfig(subtpl.ResolveOptions{
			Settings: settings.template(), Hosts: hosts, User: tplUser, HostOverrides: overrides, SubPublicDomain: h.subPublicDomain,
		})
		links = subtpl.LinkList(resolved, false)
	}
	return ojson.New(
		"isFound", true,
		"user", ojson.New(
			"shortUuid", tplUser.ShortUUID,
			"daysLeft", daysLeft(tplUser.ExpireAt),
			"trafficUsed", prettyBytes(tplUser.UsedTrafficBytes),
			"trafficLimit", prettyBytes(tplUser.TrafficLimitBytes),
			"lifetimeTrafficUsed", prettyBytes(tplUser.LifetimeUsedTrafficBytes),
			"lifetimeTrafficUsedBytes", formatInt(tplUser.LifetimeUsedTrafficBytes),
			"trafficLimitBytes", formatInt(tplUser.TrafficLimitBytes),
			"trafficUsedBytes", formatInt(tplUser.UsedTrafficBytes),
			"username", tplUser.Username,
			"expiresAt", js.ISOString(tplUser.ExpireAt),
			"isActive", tplUser.Status == "ACTIVE",
			"userStatus", tplUser.Status,
			"trafficLimitStrategy", tplUser.TrafficLimitStrategy,
		),
		"links", stringsToAny(links),
		"ssConfLinks", ojson.New(),
		"subscriptionUrl", "https://"+h.subPublicDomain+"/"+tplUser.ShortUUID,
	), nil
}

func stringsToAny(list []string) []any {
	out := make([]any, len(list))
	for i, value := range list {
		out[i] = value
	}
	return out
}
