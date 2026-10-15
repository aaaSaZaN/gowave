package subscription

import (
	"errors"
	"net/http"
	"regexp"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"remnawave-go/internal/database"
	"remnawave-go/internal/users"
)

var hwidPattern = regexp.MustCompile(`^[a-zA-Z0-9=-]{10,64}$`)

const maxHeaderLength = 512

// truncateHeader ports truncate-header.util: the first value, cut to 512
// UTF-16 units; nil when the header is absent.
func truncateHeader(r *http.Request, name string) *string {
	values := r.Header.Values(name)
	if len(values) == 0 {
		return nil
	}
	value := truncateUTF16(values[0], maxHeaderLength)
	return &value
}

func truncateUTF16(s string, limit int) string {
	units := 0
	for i, r := range s {
		size := 1
		if r > 0xFFFF {
			size = 2
		}
		if units+size > limit {
			return s[:i]
		}
		units += size
	}
	return s
}

// hwidHeaders is HwidHeaders.
type hwidHeaders struct {
	HWID        string
	Platform    *string
	OSVersion   *string
	DeviceModel *string
	UserAgent   *string
}

// extractHwidHeaders ports extract-hwid-headers.util.
func extractHwidHeaders(r *http.Request) *hwidHeaders {
	values := r.Header.Values("X-Hwid")
	if len(values) == 0 || !hwidPattern.MatchString(values[0]) {
		return nil
	}
	return &hwidHeaders{
		HWID:        values[0],
		Platform:    truncateHeader(r, "X-Device-Os"),
		OSVersion:   truncateHeader(r, "X-Ver-Os"),
		DeviceModel: truncateHeader(r, "X-Device-Model"),
		UserAgent:   truncateHeader(r, "User-Agent"),
	}
}

// hwidCheckup is IHwidCheckupResult.
type hwidCheckup struct {
	SubscriptionAllowed bool `json:"subscriptionAllowed"`
	MaxDeviceReached    bool `json:"maxDeviceReached"`
	HwidNotSupported    bool `json:"hwidNotSupported"`
	LimitBypassed       bool `json:"limitBypassed"`
}

var (
	checkupAllowed  = hwidCheckup{SubscriptionAllowed: true}
	checkupLimited  = hwidCheckup{MaxDeviceReached: true}
	checkupBypassed = hwidCheckup{SubscriptionAllowed: true, LimitBypassed: true}
)

func optionalIP(ip string) *string {
	if ip == "" {
		return nil
	}
	return &ip
}

// upsertHwidDevice ports UpsertHwidUserDeviceCommand: absent fields keep
// their stored values and no event is emitted.
func upsertHwidDevice(db *gorm.DB, userID uint64, headers *hwidHeaders, ip string) error {
	now := time.Now().UTC()
	device := database.HwidDevice{HWID: headers.HWID, UserID: userID, Platform: headers.Platform, OSVersion: headers.OSVersion,
		DeviceModel: headers.DeviceModel, UserAgent: headers.UserAgent, RequestIP: optionalIP(ip), CreatedAt: now, UpdatedAt: now}
	columns := []string{"updated_at"}
	for column, value := range map[string]*string{"platform": device.Platform, "os_version": device.OSVersion, "device_model": device.DeviceModel, "user_agent": device.UserAgent, "request_ip": device.RequestIP} {
		if value != nil {
			columns = append(columns, column)
		}
	}
	return db.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "hwid"}, {Name: "user_id"}}, DoUpdates: clause.AssignmentColumns(columns)}).Create(&device).Error
}

// upsertHwidQuietly ports checkAndUpsertHwidUserDevice: failures are logged
// upstream and never affect the response.
func (h *Handler) upsertHwidQuietly(user *database.User, headers *hwidHeaders, ip string) {
	if headers != nil {
		_ = upsertHwidDevice(h.db, user.ID, headers, ip)
	}
}

var errLimitReached = errors.New("limit reached")

// checkHwidDeviceLimit ports SubscriptionService.checkHwidDeviceLimit.
func (h *Handler) checkHwidDeviceLimit(user *database.User, headers *hwidHeaders, settings hwidSettings, ip string) hwidCheckup {
	if user.HWIDDeviceLimit != nil && *user.HWIDDeviceLimit == 0 {
		if headers != nil {
			if err := upsertHwidDevice(h.db, user.ID, headers, ip); err != nil {
				return checkupLimited
			}
		}
		return checkupBypassed
	}
	if headers == nil {
		return hwidCheckup{HwidNotSupported: true}
	}
	var exists int64
	if err := h.db.Model(&database.HwidDevice{}).Where("hwid = ? AND user_id = ?", headers.HWID, user.ID).Count(&exists).Error; err == nil && exists > 0 {
		if err := upsertHwidDevice(h.db, user.ID, headers, ip); err != nil {
			return checkupLimited
		}
		return checkupAllowed
	}
	limit := settings.FallbackDeviceLimit
	if user.HWIDDeviceLimit != nil {
		limit = *user.HWIDDeviceLimit
	}
	existed := false
	err := h.db.Transaction(func(tx *gorm.DB) error {
		// Serialises concurrent first requests like pg_advisory_xact_lock.
		var locked database.User
		query := tx.Where("id = ?", user.ID)
		if tx.Dialector.Name() == "postgres" {
			query = query.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		if err := query.First(&locked).Error; err != nil {
			return err
		}
		var found int64
		if err := tx.Model(&database.HwidDevice{}).Where("hwid = ? AND user_id = ?", headers.HWID, user.ID).Count(&found).Error; err != nil {
			return err
		}
		if found > 0 {
			existed = true
			return nil
		}
		var count int64
		if err := tx.Model(&database.HwidDevice{}).Where("user_id = ?", user.ID).Count(&count).Error; err != nil {
			return err
		}
		if count >= int64(limit) {
			return errLimitReached
		}
		now := time.Now().UTC()
		device := database.HwidDevice{HWID: headers.HWID, UserID: user.ID, Platform: headers.Platform, OSVersion: headers.OSVersion,
			DeviceModel: headers.DeviceModel, UserAgent: headers.UserAgent, RequestIP: optionalIP(ip), CreatedAt: now, UpdatedAt: now}
		if err := tx.Create(&device).Error; err != nil {
			return err
		}
		return users.EmitHwidDevice(tx, &locked, &device, "user_hwid_devices.added")
	})
	if err != nil {
		return checkupLimited
	}
	if existed {
		if err := upsertHwidDevice(h.db, user.ID, headers, ip); err != nil {
			return checkupLimited
		}
	}
	return checkupAllowed
}
