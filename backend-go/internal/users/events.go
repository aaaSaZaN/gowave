package users

import (
	"errors"
	"remnawave-go/internal/database"
	"remnawave-go/internal/notify"

	"gorm.io/gorm"
)

// subPublicDomain is SUB_PUBLIC_DOMAIN, used for subscriptionUrl in payloads.
var subPublicDomain string

// SetSubPublicDomain configures subscription URLs in notification payloads.
func SetSubPublicDomain(domain string) { subPublicDomain = domain }

// eventPolicy selects the notifications a mutation emits per user, matching
// upstream: single-user actions notify, bulk updates do not.
type eventPolicy struct {
	// operation is emitted for every changed user ("" for none).
	operation string
	// status also emits user.enabled/disabled/limited/expired on transitions.
	status bool
}

var statusEvents = map[string]string{
	"ACTIVE":   "user.enabled",
	"DISABLED": "user.disabled",
	"LIMITED":  "user.limited",
	"EXPIRED":  "user.expired",
}

// eventData formats a user with traffic and squads loaded through tx, so it
// works inside the mutation's transaction (SQLite has one connection).
func eventData(tx *gorm.DB, user *database.User) (map[string]interface{}, error) {
	snapshot := *user
	var traffic database.UserTraffic
	if err := tx.Where("id = ?", user.ID).Limit(1).Find(&traffic).Error; err != nil {
		return nil, err
	}
	if traffic.ID != 0 {
		snapshot.Traffic = &traffic
	}
	if err := loadSquads(tx, []*database.User{&snapshot}); err != nil {
		return nil, err
	}
	return FormatUser(&snapshot, subPublicDomain), nil
}

func emitUser(tx *gorm.DB, user *database.User, names ...string) error {
	if len(names) == 0 {
		return nil
	}
	data, err := eventData(tx, user)
	if err != nil {
		return err
	}
	events := make([]notify.Event, 0, len(names))
	for _, name := range names {
		events = append(events, notify.Event{Name: name, Data: data})
	}
	return notify.Emit(tx, events...)
}

// EmitHwidDevice enqueues user_hwid_devices.* with the device and full user.
func EmitHwidDevice(tx *gorm.DB, user *database.User, device *database.HwidDevice, name string) error {
	data, err := eventData(tx, user)
	if err != nil {
		return err
	}
	snapshot := *device
	snapshot.CreatedAt, snapshot.UpdatedAt = snapshot.CreatedAt.UTC(), snapshot.UpdatedAt.UTC()
	return notify.Emit(tx, notify.Event{Name: name, Data: map[string]interface{}{"hwidUserDevice": snapshot, "user": data}})
}

// SetStatus moves users to status. When notifyEach is set every user emits
// the matching user.* status event; upstream skips them for huge batches.
func (s *Service) SetStatusFor(ids []uint64, status string, notifyEach bool) error {
	if !validStatus(status) {
		return errors.New("invalid user status")
	}
	for start := 0; start < len(ids); start += 1000 {
		end := start + 1000
		if end > len(ids) {
			end = len(ids)
		}
		mutation := func(tx *gorm.DB, u *database.User) (bool, error) { u.Status = status; return false, nil }
		if err := s.mutateNotify(byIDs(ids[start:end]), false, mutation, eventPolicy{status: notifyEach}); err != nil {
			return err
		}
	}
	return nil
}

// EmitForUsers sends one event per user with shared meta, in batches.
func EmitForUsers(db *gorm.DB, ids []uint64, name string, meta interface{}, skipTelegram bool) error {
	for start := 0; start < len(ids); start += 500 {
		end := start + 500
		if end > len(ids) {
			end = len(ids)
		}
		err := db.Transaction(func(tx *gorm.DB) error {
			var batch []database.User
			if err := tx.Where("id IN ?", ids[start:end]).Find(&batch).Error; err != nil {
				return err
			}
			for i := range batch {
				data, err := eventData(tx, &batch[i])
				if err != nil {
					return err
				}
				if err := notify.Emit(tx, notify.Event{Name: name, Data: data, Meta: meta, SkipTelegram: skipTelegram}); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// EventData formats a user for notifications using tx for related rows.
func EventData(tx *gorm.DB, user *database.User) (map[string]interface{}, error) {
	return eventData(tx, user)
}
