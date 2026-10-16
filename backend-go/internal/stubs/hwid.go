package stubs

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"gowave/internal/database"
	"gowave/internal/httpx"
	"gowave/internal/users"

	"gorm.io/gorm"
)

var (
	errHwidExists   = errors.New("hwid exists")
	errHwidLimit    = errors.New("hwid limit")
	errHwidNotFound = errors.New("hwid not found")
)

func (s *StubHandler) userDevices(w http.ResponseWriter, userID uint64) {
	var devices []database.HwidDevice
	s.db.Where("user_id = ?", userID).Order("created_at DESC").Find(&devices)
	if devices == nil {
		devices = []database.HwidDevice{}
	}
	httpx.Response(w, map[string]interface{}{"total": len(devices), "devices": devices})
}

// deviceLimit resolves the effective HWID policy like upstream: global
// settings, replaced by the user's external squad settings when present.
func (s *StubHandler) deviceLimit(tx *gorm.DB, user *database.User) (enabled bool, limit int, err error) {
	var settings struct {
		Enabled bool `json:"enabled"`
		Limit   int  `json:"fallbackDeviceLimit"`
	}
	global := database.EnsureSubscriptionSetting(tx)
	if global != nil {
		_ = json.Unmarshal([]byte(global.HwidSettings), &settings)
	}
	if user.ExternalSquadUUID != nil {
		var squad database.ExternalSquad
		if err := tx.Where("uuid = ?", *user.ExternalSquadUUID).First(&squad).Error; err == nil && squad.HwidSettings != "" && squad.HwidSettings != "{}" && squad.HwidSettings != "null" {
			_ = json.Unmarshal([]byte(squad.HwidSettings), &settings)
		}
	}
	limit = settings.Limit
	if user.HWIDDeviceLimit != nil {
		limit = *user.HWIDDeviceLimit
	}
	return settings.Enabled, limit, nil
}

func (s *StubHandler) CreateHwidDevice(w http.ResponseWriter, r *http.Request) {
	var req struct {
		HWID        string  `json:"hwid"`
		UserID      uint64  `json:"userId"`
		Platform    *string `json:"platform"`
		OSVersion   *string `json:"osVersion"`
		DeviceModel *string `json:"deviceModel"`
		UserAgent   *string `json:"userAgent"`
		RequestIP   *string `json:"requestIp"`
	}
	if err := httpx.Decode(r, 16<<10, &req); err != nil || req.HWID == "" || req.UserID == 0 {
		httpx.Error(w, r, http.StatusBadRequest, "E000", "hwid and userId are required")
		return
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var user database.User
		if err := tx.Where("id = ?", req.UserID).First(&user).Error; err != nil {
			return err
		}
		var existing int64
		if err := tx.Model(&database.HwidDevice{}).Where("hwid = ? AND user_id = ?", req.HWID, user.ID).Count(&existing).Error; err != nil {
			return err
		}
		if existing > 0 {
			return errHwidExists
		}
		enabled, limit, err := s.deviceLimit(tx, &user)
		if err != nil {
			return err
		}
		if enabled {
			var count int64
			if err := tx.Model(&database.HwidDevice{}).Where("user_id = ?", user.ID).Count(&count).Error; err != nil {
				return err
			}
			if count >= int64(limit) {
				return errHwidLimit
			}
		}
		now := time.Now().UTC()
		device := database.HwidDevice{HWID: req.HWID, UserID: user.ID, Platform: req.Platform, OSVersion: req.OSVersion, DeviceModel: req.DeviceModel, UserAgent: req.UserAgent, RequestIP: req.RequestIP, CreatedAt: now, UpdatedAt: now}
		if err := tx.Create(&device).Error; err != nil {
			return err
		}
		return users.EmitHwidDevice(tx, &user, &device, "user_hwid_devices.added")
	})
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		httpx.Error(w, r, http.StatusNotFound, "A025", "User not found")
	case errors.Is(err, errHwidExists):
		httpx.Error(w, r, http.StatusBadRequest, "A098", "User HWID device already exists")
	case errors.Is(err, errHwidLimit):
		httpx.Error(w, r, http.StatusBadRequest, "A099", "User HWID device limit reached")
	case err != nil:
		httpx.Error(w, r, http.StatusInternalServerError, "A096", "Create HWID user device error")
	default:
		s.userDevices(w, req.UserID)
	}
}

func (s *StubHandler) DeleteHwidDevice(w http.ResponseWriter, r *http.Request) {
	var req struct {
		HWID   string `json:"hwid"`
		UserID uint64 `json:"userId"`
	}
	if err := httpx.Decode(r, 4096, &req); err != nil || req.HWID == "" || req.UserID == 0 {
		httpx.Error(w, r, http.StatusBadRequest, "E000", "hwid and userId are required")
		return
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var user database.User
		if err := tx.Where("id = ?", req.UserID).First(&user).Error; err != nil {
			return err
		}
		var device database.HwidDevice
		if err := tx.Where("hwid = ? AND user_id = ?", req.HWID, user.ID).First(&device).Error; err != nil {
			return errHwidNotFound
		}
		if err := tx.Where("hwid = ? AND user_id = ?", req.HWID, user.ID).Delete(&database.HwidDevice{}).Error; err != nil {
			return err
		}
		return users.EmitHwidDevice(tx, &user, &device, "user_hwid_devices.deleted")
	})
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		httpx.Error(w, r, http.StatusNotFound, "A025", "User not found")
	case errors.Is(err, errHwidNotFound):
		httpx.Error(w, r, http.StatusNotFound, "A204", "HWID device not found")
	case err != nil:
		httpx.Error(w, r, http.StatusInternalServerError, "A101", "Delete HWID user device error")
	default:
		s.userDevices(w, req.UserID)
	}
}

func (s *StubHandler) DeleteAllHwidDevices(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID uint64 `json:"userId"`
	}
	if err := httpx.Decode(r, 4096, &req); err != nil || req.UserID == 0 {
		httpx.Error(w, r, http.StatusBadRequest, "E000", "userId is required")
		return
	}
	var user database.User
	if err := s.db.Where("id = ?", req.UserID).First(&user).Error; err != nil {
		httpx.Error(w, r, http.StatusNotFound, "A025", "User not found")
		return
	}
	if err := s.db.Where("user_id = ?", user.ID).Delete(&database.HwidDevice{}).Error; err != nil {
		httpx.Error(w, r, http.StatusInternalServerError, "A155", "Delete HWID user devices error")
		return
	}
	s.userDevices(w, user.ID)
}
