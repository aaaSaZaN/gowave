package users

import (
	"errors"
	"fmt"
	"remnawave-go/internal/database"
	"remnawave-go/internal/jobs"
	"remnawave-go/internal/notify"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type userMutation func(*gorm.DB, *database.User) (bool, error)

// mutate keeps database state and node work in the same transaction for every
// single, bulk, and scheduled operation. A deletion retains its credentials in
// the outbox tombstone until offline nodes have processed the removal.
func (s *Service) mutate(query func(*gorm.DB) *gorm.DB, required bool, mutation userMutation) error {
	return s.mutateNotify(query, required, mutation, eventPolicy{})
}

func (s *Service) mutateNotify(query func(*gorm.DB) *gorm.DB, required bool, mutation userMutation, policy eventPolicy) error {
	var changed []database.User
	deleted := make(map[uint64]bool)
	err := s.db.Transaction(func(tx *gorm.DB) error {
		selectQuery := query(tx.Model(&database.User{}))
		if tx.Dialector.Name() == "postgres" {
			selectQuery = selectQuery.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		if err := selectQuery.Find(&changed).Error; err != nil {
			return err
		}
		if required && len(changed) == 0 {
			return gorm.ErrRecordNotFound
		}
		for i := range changed {
			user := &changed[i]
			previous := user.VlessUUID
			previousStatus := user.Status
			// A deletion's payload must be captured before rows disappear.
			var deletedData map[string]interface{}
			if policy.operation == "user.deleted" {
				data, err := eventData(tx, user)
				if err != nil {
					return err
				}
				deletedData = data
			}
			removed, err := mutation(tx, user)
			if err != nil {
				return err
			}
			user.Revision++
			user.UpdatedAt = time.Now().UTC()
			if !removed {
				if err := tx.Omit(clause.Associations).Save(user).Error; err != nil {
					return err
				}
			}
			deleted[user.ID] = removed
			if removed && deletedData != nil {
				if err := notify.Emit(tx, notify.Event{Name: "user.deleted", Data: deletedData}); err != nil {
					return err
				}
			} else if !removed {
				var names []string
				if policy.status && previousStatus != user.Status {
					names = append(names, statusEvents[user.Status])
				}
				if policy.operation != "" {
					names = append(names, policy.operation)
				}
				if err := emitUser(tx, user, names...); err != nil {
					return err
				}
			}
			if err := jobs.EnqueueUser(tx, user, &previous, removed); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	if s.syncer != nil {
		for i := range changed {
			u := &changed[i]
			if deleted[u.ID] || u.Status != "ACTIVE" {
				s.syncer.RemoveUserFromNodes(u)
			} else {
				s.syncer.SyncUserToNodes(u, nil)
			}
		}
	}
	return nil
}

func byIDs(ids []uint64) func(*gorm.DB) *gorm.DB {
	return func(tx *gorm.DB) *gorm.DB { return tx.Where("id IN ?", ids) }
}
func allUsers(tx *gorm.DB) *gorm.DB { return tx }

func deleteUser(tx *gorm.DB, user *database.User) (bool, error) {
	if err := tx.Where("user_id = ?", user.ID).Delete(&database.InternalSquadMember{}).Error; err != nil {
		return false, err
	}
	if err := tx.Where("id = ?", user.ID).Delete(&database.UserTraffic{}).Error; err != nil {
		return false, err
	}
	return true, tx.Where("id = ?", user.ID).Delete(&database.User{}).Error
}
func (s *Service) Delete(id uint64) error {
	return s.mutateNotify(byIDs([]uint64{id}), true, deleteUser, eventPolicy{operation: "user.deleted"})
}
func (s *Service) BulkDelete(ids []uint64) error {
	return s.mutateNotify(byIDs(ids), false, deleteUser, eventPolicy{operation: "user.deleted"})
}
func (s *Service) BulkDeleteByStatus(status string) error {
	if !validStatus(status) {
		return fmt.Errorf("invalid user status")
	}
	return s.mutate(func(tx *gorm.DB) *gorm.DB { return tx.Where("status = ?", status) }, false, deleteUser)
}

func resetUserTraffic(tx *gorm.DB, user *database.User) (bool, error) {
	if err := tx.Model(&database.UserTraffic{}).Where("id = ?", user.ID).Update("used_traffic_bytes", 0).Error; err != nil {
		return false, err
	}
	now := time.Now().UTC()
	user.LastTrafficResetAt = &now
	user.LastTriggeredThreshold = 0
	if user.Status == "LIMITED" {
		if user.ExpireAt.After(now) {
			user.Status = "ACTIVE"
		} else {
			user.Status = "EXPIRED"
		}
	}
	return false, nil
}
func (s *Service) ResetTraffic(id uint64) error {
	return s.mutateNotify(byIDs([]uint64{id}), true, resetUserTraffic, eventPolicy{operation: "user.traffic_reset", status: true})
}
func (s *Service) BulkResetTraffic(ids []uint64) error {
	return s.mutate(byIDs(ids), false, resetUserTraffic)
}
func (s *Service) BulkAllResetTraffic() error { return s.mutate(allUsers, false, resetUserTraffic) }
func (s *Service) SetStatus(id uint64, status string) error {
	if !validStatus(status) {
		return fmt.Errorf("invalid user status")
	}
	return s.mutate(byIDs([]uint64{id}), true, func(tx *gorm.DB, u *database.User) (bool, error) { u.Status = status; return false, nil })
}

var (
	ErrAlreadyEnabled  = errors.New("user already enabled")
	ErrAlreadyDisabled = errors.New("user already disabled")
)

// Enable activates a user; like upstream it refuses an already ACTIVE user.
func (s *Service) Enable(id uint64) error {
	return s.mutateNotify(byIDs([]uint64{id}), true, func(tx *gorm.DB, u *database.User) (bool, error) {
		if u.Status == "ACTIVE" {
			return false, ErrAlreadyEnabled
		}
		u.Status = "ACTIVE"
		return false, nil
	}, eventPolicy{status: true})
}

// Disable deactivates a user; like upstream it refuses an already DISABLED user.
func (s *Service) Disable(id uint64) error {
	return s.mutateNotify(byIDs([]uint64{id}), true, func(tx *gorm.DB, u *database.User) (bool, error) {
		if u.Status == "DISABLED" {
			return false, ErrAlreadyDisabled
		}
		u.Status = "DISABLED"
		return false, nil
	}, eventPolicy{status: true})
}

func validStatus(status string) bool {
	return status == "ACTIVE" || status == "LIMITED" || status == "EXPIRED" || status == "DISABLED"
}

func extendMutation(days int, single bool) userMutation {
	return func(tx *gorm.DB, u *database.User) (bool, error) {
		base := u.ExpireAt
		now := time.Now().UTC()
		// A single extension starts from now when the user has already lapsed;
		// bulk extension adds to the stored date like upstream's SQL update.
		if single && base.Before(now) {
			base = now
		}
		u.ExpireAt = base.AddDate(0, 0, days)
		if u.Status == "EXPIRED" && u.ExpireAt.After(now) {
			u.Status = "ACTIVE"
		}
		return false, nil
	}
}
func validDays(days int) error {
	if days < 1 || days > 9999 {
		return fmt.Errorf("days must be between 1 and 9999")
	}
	return nil
}
func (s *Service) Extend(id uint64, days int) (*database.User, error) {
	if err := validDays(days); err != nil {
		return nil, err
	}
	if err := s.mutateNotify(byIDs([]uint64{id}), true, extendMutation(days, true), eventPolicy{operation: "user.modified"}); err != nil {
		return nil, err
	}
	return s.GetByID(id)
}
func (s *Service) BulkExtendExpirationDate(ids []uint64, days int) error {
	if err := validDays(days); err != nil {
		return err
	}
	return s.mutate(byIDs(ids), false, extendMutation(days, false))
}
func (s *Service) BulkAllExtendExpirationDate(days int) error {
	if err := validDays(days); err != nil {
		return err
	}
	return s.mutate(allUsers, false, extendMutation(days, false))
}

func (s *Service) updateMany(query func(*gorm.DB) *gorm.DB, raw map[string]interface{}) error {
	fields, squads, hasSquads := normalizeUserFields(raw)
	if status, ok := fields["status"]; ok {
		if value, ok := status.(string); !ok || !validStatus(value) {
			return fmt.Errorf("invalid user status")
		}
	}
	return s.mutate(query, false, func(tx *gorm.DB, u *database.User) (bool, error) {
		if len(fields) > 0 {
			if err := tx.Model(&database.User{}).Where("id = ?", u.ID).Updates(fields).Error; err != nil {
				return false, err
			}
		}
		if hasSquads {
			if err := tx.Where("user_id = ?", u.ID).Delete(&database.InternalSquadMember{}).Error; err != nil {
				return false, err
			}
			seen := map[string]bool{}
			for _, id := range squads {
				if seen[id] {
					continue
				}
				seen[id] = true
				if err := tx.Create(&database.InternalSquadMember{UserID: u.ID, InternalSquadUUID: id}).Error; err != nil {
					return false, err
				}
			}
		}
		return false, tx.Where("id = ?", u.ID).First(u).Error
	})
}
func (s *Service) BulkUpdate(ids []uint64, fields map[string]interface{}) error {
	return s.updateMany(byIDs(ids), fields)
}
func (s *Service) BulkAllUpdate(fields map[string]interface{}) error {
	return s.updateMany(allUsers, fields)
}
func (s *Service) BulkUpdateSquads(ids []uint64, squads []string) error {
	return s.updateMany(byIDs(ids), map[string]interface{}{"activeInternalSquads": squads})
}

func revokeMutation(passwordsOnly bool, shortUUID string) userMutation {
	return func(tx *gorm.DB, u *database.User) (bool, error) {
		u.VlessUUID = uuid.NewString()
		u.TrojanPassword = NewPassword()
		u.SsPassword = NewPassword()
		if !passwordsOnly {
			u.ShortUUID = shortUUID
			if u.ShortUUID == "" {
				u.ShortUUID = NewShortUUID()
			}
		}
		now := time.Now().UTC()
		u.SubRevokedAt = &now
		// Upstream keeps HWID registrations; only credentials rotate.
		return false, nil
	}
}
func (s *Service) Revoke(id uint64, passwordsOnly bool, shortUUID string) (*database.User, error) {
	if shortUUID != "" && (len(shortUUID) < 16 || len(shortUUID) > 64) {
		return nil, fmt.Errorf("shortUuid must be between 16 and 64 characters")
	}
	if err := s.mutateNotify(byIDs([]uint64{id}), true, revokeMutation(passwordsOnly, shortUUID), eventPolicy{operation: "user.revoked"}); err != nil {
		return nil, err
	}
	return s.GetByID(id)
}
func (s *Service) BulkRevokeSubscription(ids []uint64) error {
	return s.mutate(byIDs(ids), false, revokeMutation(false, ""))
}
