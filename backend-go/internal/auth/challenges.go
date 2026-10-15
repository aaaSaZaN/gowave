package auth

import (
	"errors"
	"remnawave-go/internal/database"
	"time"

	"gorm.io/gorm"
)

// ConsumeChallenge makes replay impossible, including simultaneous requests.
func ConsumeChallenge(db *gorm.DB, id, purpose, adminUUID string) (*database.AuthChallenge, error) {
	var record database.AuthChallenge
	err := db.Transaction(func(tx *gorm.DB) error {
		query := tx.Where("id = ? AND purpose = ? AND expires_at > ?", id, purpose, time.Now().UTC())
		if adminUUID != "" {
			query = query.Where("admin_uuid = ?", adminUUID)
		}
		if err := query.First(&record).Error; err != nil {
			return err
		}
		if !record.ExpiresAt.After(time.Now()) {
			return errors.New("challenge expired")
		}
		result := tx.Where("id = ? AND purpose = ? AND expires_at > ?", record.ID, purpose, time.Now().UTC()).Delete(&database.AuthChallenge{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("challenge already consumed")
		}
		return nil
	})
	return &record, err
}
