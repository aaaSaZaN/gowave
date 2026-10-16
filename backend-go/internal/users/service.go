package users

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"gowave/internal/database"
	"gowave/internal/jobs"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type NodeUserSyncer interface {
	SyncUserToNodes(user *database.User, prevVlessUUID *string)
	RemoveUserFromNodes(user *database.User)
}

type Service struct {
	db     *gorm.DB
	syncer NodeUserSyncer
}

func NewService(db *gorm.DB, syncer ...NodeUserSyncer) *Service {
	var s NodeUserSyncer
	if len(syncer) > 0 {
		s = syncer[0]
	}
	return &Service{db: db, syncer: s}
}

func (s *Service) SetSyncer(syncer NodeUserSyncer) {
	s.syncer = syncer
}

const alphabet = "0123456789ABCDEFGHJKLMNPQRSTUVWXYZ_abcdefghjkmnopqrstuvwxyz-"

func GenerateShortUUID(length int) string {
	if length <= 0 {
		length = 16
	}
	bytes := make([]byte, length)
	for i := 0; i < length; i++ {
		num, _ := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
		bytes[i] = alphabet[num.Int64()]
	}
	return string(bytes)
}

// GenerateRandomPassword draws from upstream's nanoid alphabet.
func GenerateRandomPassword(length int) string {
	return GenerateShortUUID(length)
}

// shortUUIDLength is SHORT_UUID_LENGTH; set once at startup.
var shortUUIDLength = 16

// SetShortUUIDLength applies SHORT_UUID_LENGTH (16-64) to generated ids.
func SetShortUUIDLength(length int) {
	if length >= 16 && length <= 64 {
		shortUUIDLength = length
	}
}

// NewShortUUID generates a subscription identifier like upstream's nanoid
// method with the configured length.
func NewShortUUID() string { return GenerateShortUUID(shortUUIDLength) }

// NewPassword generates a 32-character protocol password like upstream.
func NewPassword() string { return GenerateRandomPassword(32) }

type CreateUserDTO struct {
	Username             string    `json:"username"`
	Status               string    `json:"status"`
	ShortUUID            string    `json:"shortUuid"`
	TrafficLimitBytes    uint64    `json:"trafficLimitBytes"`
	TrafficLimitStrategy string    `json:"trafficLimitStrategy"`
	ExpireAt             time.Time `json:"expireAt"`
	Description          *string   `json:"description"`
	Tag                  *string   `json:"tag"`
	TelegramID           *int64    `json:"telegramId"`
	Email                *string   `json:"email"`
	HWIDDeviceLimit      *int      `json:"hwidDeviceLimit"`
	ExternalSquadUUID    *string   `json:"externalSquadUuid"`
	TrojanPassword       string    `json:"trojanPassword"`
	VlessUUID            string    `json:"vlessUuid"`
	SsPassword           string    `json:"ssPassword"`
	ActiveInternalSquads []string  `json:"activeInternalSquads"`
}

type squadMemberRow struct {
	UserID uint64 `gorm:"column:user_id"`
	UUID   string `gorm:"column:uuid"`
	Name   string `gorm:"column:name"`
}

func (s *Service) loadSquadsForUsers(users []*database.User) error {
	return loadSquads(s.db, users)
}

func loadSquads(db *gorm.DB, users []*database.User) error {
	if len(users) == 0 {
		return nil
	}
	userIDs := make([]uint64, 0, len(users))
	for _, u := range users {
		u.ActiveInternalSquads = []database.InternalSquad{}
		userIDs = append(userIDs, u.ID)
	}

	var rows []squadMemberRow
	err := db.Table("internal_squad_members").
		Select("internal_squad_members.user_id, internal_squads.uuid, internal_squads.name").
		Joins("JOIN internal_squads ON internal_squads.uuid = internal_squad_members.internal_squad_uuid").
		Where("internal_squad_members.user_id IN ?", userIDs).
		Scan(&rows).Error
	if err != nil {
		return err
	}

	squadsMap := make(map[uint64][]database.InternalSquad)
	for _, r := range rows {
		squadsMap[r.UserID] = append(squadsMap[r.UserID], database.InternalSquad{
			UUID: r.UUID,
			Name: r.Name,
		})
	}

	for _, u := range users {
		if sq, ok := squadsMap[u.ID]; ok {
			u.ActiveInternalSquads = sq
		}
	}
	return nil
}

func (s *Service) Create(dto CreateUserDTO) (*database.User, error) {
	if dto.TrafficLimitStrategy == "" {
		dto.TrafficLimitStrategy = "NO_RESET"
	}
	if dto.Status == "" {
		dto.Status = "ACTIVE"
	}
	if dto.ShortUUID == "" {
		dto.ShortUUID = NewShortUUID()
	}
	if dto.TrojanPassword == "" {
		dto.TrojanPassword = NewPassword()
	}
	if dto.VlessUUID == "" {
		dto.VlessUUID = uuid.NewString()
	}
	if dto.SsPassword == "" {
		dto.SsPassword = NewPassword()
	}

	var emailStr string
	if dto.Email != nil {
		emailStr = strings.TrimSpace(*dto.Email)
	}
	var tagStr string
	if dto.Tag != nil {
		tagStr = strings.TrimSpace(*dto.Tag)
	}
	var descStr string
	if dto.Description != nil {
		descStr = strings.TrimSpace(*dto.Description)
	}

	now := time.Now().UTC()
	user := &database.User{
		ShortUUID:            dto.ShortUUID,
		Username:             dto.Username,
		Status:               dto.Status,
		TrafficLimitBytes:    dto.TrafficLimitBytes,
		TrafficLimitStrategy: dto.TrafficLimitStrategy,
		ExpireAt:             dto.ExpireAt.UTC(),
		TrojanPassword:       dto.TrojanPassword,
		VlessUUID:            dto.VlessUUID,
		SsPassword:           dto.SsPassword,
		Description:          descStr,
		Tag:                  tagStr,
		TelegramID:           dto.TelegramID,
		Email:                emailStr,
		HWIDDeviceLimit:      dto.HWIDDeviceLimit,
		ExternalSquadUUID:    dto.ExternalSquadUUID,
		CreatedAt:            now,
		UpdatedAt:            now,
	}

	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(user).Error; err != nil {
			return err
		}
		traffic := &database.UserTraffic{
			ID: user.ID,
		}
		if err := tx.Create(traffic).Error; err != nil {
			return err
		}
		for _, squadUUID := range dto.ActiveInternalSquads {
			squadUUID = strings.TrimSpace(squadUUID)
			if squadUUID != "" {
				member := database.InternalSquadMember{
					InternalSquadUUID: squadUUID,
					UserID:            user.ID,
				}
				if err := tx.Create(&member).Error; err != nil {
					return err
				}
			}
		}
		if err := emitUser(tx, user, "user.created"); err != nil {
			return err
		}
		return jobs.EnqueueUser(tx, user, nil, false)
	})

	if err != nil {
		return nil, err
	}

	_ = s.loadSquadsForUsers([]*database.User{user})
	if s.syncer != nil && strings.ToUpper(user.Status) == "ACTIVE" {
		s.syncer.SyncUserToNodes(user, nil)
	}
	return user, nil
}

type UserFilter struct {
	ID    string      `json:"id"`
	Value interface{} `json:"value"`
}

type UserSorting struct {
	ID   string `json:"id"`
	Desc bool   `json:"desc"`
}

type GetUsersQuery struct {
	Start   int
	Size    int
	Status  string
	Tag     string
	Filters []UserFilter
	Sorting []UserSorting
}

func (s *Service) GetAll(q GetUsersQuery) ([]*database.User, int64, error) {
	dbQuery := s.db.Model(&database.User{}).Preload("Traffic")
	countQuery := s.db.Model(&database.User{})

	if q.Status != "" {
		dbQuery = dbQuery.Where("users.status = ?", q.Status)
		countQuery = countQuery.Where("users.status = ?", q.Status)
	}
	if q.Tag != "" {
		dbQuery = dbQuery.Where("users.tag = ?", q.Tag)
		countQuery = countQuery.Where("users.tag = ?", q.Tag)
	}

	for _, f := range q.Filters {
		if f.ID == "" || f.Value == nil {
			continue
		}
		switch f.ID {
		case "status":
			switch v := f.Value.(type) {
			case []interface{}:
				if len(v) > 0 {
					var statuses []string
					for _, item := range v {
						if str, ok := item.(string); ok && str != "" {
							statuses = append(statuses, str)
						}
					}
					if len(statuses) > 0 {
						dbQuery = dbQuery.Where("users.status IN ?", statuses)
						countQuery = countQuery.Where("users.status IN ?", statuses)
					}
				}
			case string:
				if v != "" {
					dbQuery = dbQuery.Where("users.status = ?", v)
					countQuery = countQuery.Where("users.status = ?", v)
				}
			}
		case "tag":
			switch v := f.Value.(type) {
			case []interface{}:
				if len(v) > 0 {
					var tags []string
					for _, item := range v {
						if str, ok := item.(string); ok && str != "" {
							tags = append(tags, str)
						}
					}
					if len(tags) > 0 {
						dbQuery = dbQuery.Where("users.tag IN ?", tags)
						countQuery = countQuery.Where("users.tag IN ?", tags)
					}
				}
			case string:
				if v != "" {
					dbQuery = dbQuery.Where("users.tag = ?", v)
					countQuery = countQuery.Where("users.tag = ?", v)
				}
			}
		case "username":
			if str, ok := f.Value.(string); ok && str != "" {
				dbQuery = dbQuery.Where("LOWER(users.username) LIKE LOWER(?)", "%"+str+"%")
				countQuery = countQuery.Where("LOWER(users.username) LIKE LOWER(?)", "%"+str+"%")
			}
		case "shortUuid":
			if str, ok := f.Value.(string); ok && str != "" {
				dbQuery = dbQuery.Where("LOWER(users.short_uuid) LIKE LOWER(?)", "%"+str+"%")
				countQuery = countQuery.Where("LOWER(users.short_uuid) LIKE LOWER(?)", "%"+str+"%")
			}
		case "id":
			valStr := fmt.Sprintf("%v", f.Value)
			if valStr != "" {
				dbQuery = dbQuery.Where("CAST(users.id AS TEXT) LIKE ?", "%"+valStr+"%")
				countQuery = countQuery.Where("CAST(users.id AS TEXT) LIKE ?", "%"+valStr+"%")
			}
		case "email":
			if str, ok := f.Value.(string); ok && str != "" {
				dbQuery = dbQuery.Where("LOWER(users.email) LIKE LOWER(?)", "%"+str+"%")
				countQuery = countQuery.Where("LOWER(users.email) LIKE LOWER(?)", "%"+str+"%")
			}
		case "description":
			if str, ok := f.Value.(string); ok && str != "" {
				dbQuery = dbQuery.Where("LOWER(users.description) LIKE LOWER(?)", "%"+str+"%")
				countQuery = countQuery.Where("LOWER(users.description) LIKE LOWER(?)", "%"+str+"%")
			}
		case "telegramId":
			valStr := fmt.Sprintf("%v", f.Value)
			if valStr != "" {
				dbQuery = dbQuery.Where("CAST(users.telegram_id AS TEXT) LIKE ?", "%"+valStr+"%")
				countQuery = countQuery.Where("CAST(users.telegram_id AS TEXT) LIKE ?", "%"+valStr+"%")
			}
		}
	}

	var total int64
	if err := countQuery.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	hasCustomSort := false
	for _, sort := range q.Sorting {
		col := ""
		switch sort.ID {
		case "id":
			col = "users.id"
		case "username":
			col = "users.username"
		case "status":
			col = "users.status"
		case "trafficLimitBytes":
			col = "users.traffic_limit_bytes"
		case "expireAt":
			col = "users.expire_at"
		case "createdAt":
			col = "users.created_at"
		case "updatedAt":
			col = "users.updated_at"
		case "tag":
			col = "users.tag"
		}
		if col != "" {
			dir := "ASC"
			if sort.Desc {
				dir = "DESC"
			}
			dbQuery = dbQuery.Order(fmt.Sprintf("%s %s", col, dir))
			hasCustomSort = true
		}
	}
	if !hasCustomSort {
		dbQuery = dbQuery.Order("users.id DESC")
	}

	size := q.Size
	if size <= 0 {
		size = 25
	}
	if size > 1000 {
		size = 1000
	}
	start := q.Start
	if start < 0 {
		start = 0
	}

	var users []*database.User
	err := dbQuery.Offset(start).Limit(size).Find(&users).Error
	if err != nil {
		return nil, 0, err
	}
	_ = s.loadSquadsForUsers(users)
	return users, total, nil
}

type StreamQuery struct {
	Cursor               *uint64
	Size                 int
	Status               string
	TrafficLimitStrategy string
	TelegramID           *int64
	Email                string
	Tag                  string
	ExternalSquadUUID    string
}

func (s *Service) GetStream(q StreamQuery) ([]*database.User, *string, bool, error) {
	size := q.Size
	if size <= 0 {
		size = 250
	}
	if size > 1000 {
		size = 1000
	}

	query := s.db.Model(&database.User{}).Preload("Traffic")
	if q.Cursor != nil {
		query = query.Where("id > ?", *q.Cursor)
	}
	if q.Status != "" {
		query = query.Where("status = ?", q.Status)
	}
	if q.TrafficLimitStrategy != "" {
		query = query.Where("traffic_limit_strategy = ?", q.TrafficLimitStrategy)
	}
	if q.TelegramID != nil {
		query = query.Where("telegram_id = ?", *q.TelegramID)
	}
	if q.Email != "" {
		query = query.Where("email = ?", q.Email)
	}
	if q.Tag != "" {
		query = query.Where("tag = ?", q.Tag)
	}
	if q.ExternalSquadUUID != "" {
		query = query.Where("external_squad_uuid = ?", q.ExternalSquadUUID)
	}

	var users []*database.User
	err := query.Order("id ASC").Limit(size + 1).Find(&users).Error
	if err != nil {
		return nil, nil, false, err
	}

	hasMore := len(users) > size
	if hasMore {
		users = users[:size]
	}

	_ = s.loadSquadsForUsers(users)

	var nextCursor *string
	if hasMore && len(users) > 0 {
		c := strconv.FormatUint(users[len(users)-1].ID, 10)
		nextCursor = &c
	}

	return users, nextCursor, hasMore, nil
}

func (s *Service) GetTags() ([]string, error) {
	var tags []string
	err := s.db.Model(&database.User{}).
		Where("tag IS NOT NULL AND tag != ''").
		Distinct("tag").
		Pluck("tag", &tags).Error
	if err != nil {
		return nil, err
	}
	if tags == nil {
		tags = []string{}
	}
	return tags, nil
}

func (s *Service) GetByID(id uint64) (*database.User, error) {
	var user database.User
	err := s.db.Preload("Traffic").First(&user, id).Error
	if err != nil {
		return nil, err
	}
	_ = s.loadSquadsForUsers([]*database.User{&user})
	return &user, nil
}

func (s *Service) GetByShortUUID(shortUUID string) (*database.User, error) {
	var user database.User
	err := s.db.Preload("Traffic").Where("short_uuid = ?", shortUUID).First(&user).Error
	if err != nil {
		return nil, err
	}
	_ = s.loadSquadsForUsers([]*database.User{&user})
	return &user, nil
}

func (s *Service) GetByUsername(username string) (*database.User, error) {
	var user database.User
	err := s.db.Preload("Traffic").Where("username = ?", username).First(&user).Error
	if err != nil {
		return nil, err
	}
	_ = s.loadSquadsForUsers([]*database.User{&user})
	return &user, nil
}

func normalizeUserFields(raw map[string]interface{}) (map[string]interface{}, []string, bool) {
	updates := make(map[string]interface{})
	var activeSquads []string
	hasSquads := false

	for k, v := range raw {
		switch k {
		case "username":
			updates["username"] = v
		case "status":
			updates["status"] = v
		case "trafficLimitBytes":
			updates["traffic_limit_bytes"] = v
		case "trafficLimitStrategy":
			updates["traffic_limit_strategy"] = v
		case "expireAt":
			if str, ok := v.(string); ok {
				if t, err := time.Parse(time.RFC3339Nano, str); err == nil {
					updates["expire_at"] = t.UTC()
				} else if t, err := time.Parse(time.RFC3339, str); err == nil {
					updates["expire_at"] = t.UTC()
				} else {
					updates["expire_at"] = str
				}
			} else {
				updates["expire_at"] = v
			}
		case "description":
			updates["description"] = v
		case "tag":
			updates["tag"] = v
		case "telegramId":
			updates["telegram_id"] = v
		case "email":
			updates["email"] = v
		case "hwidDeviceLimit":
			updates["hwid_device_limit"] = v
		case "externalSquadUuid":
			updates["external_squad_uuid"] = v
		case "trojanPassword":
			updates["trojan_password"] = v
		case "vlessUuid":
			updates["vless_uuid"] = v
		case "ssPassword":
			updates["ss_password"] = v
		case "activeInternalSquads":
			hasSquads = true
			if list, ok := v.([]interface{}); ok {
				for _, item := range list {
					if str, ok := item.(string); ok && str != "" {
						activeSquads = append(activeSquads, str)
					}
				}
			} else if list, ok := v.([]string); ok {
				activeSquads = list
			}
		}
	}
	return updates, activeSquads, hasSquads
}

func (s *Service) Update(id uint64, updates map[string]interface{}) (*database.User, error) {
	prevUser, previousErr := s.GetByID(id)
	if previousErr != nil {
		return nil, previousErr
	}
	var prevVlessUUID *string
	if prevUser != nil {
		prevVlessUUID = &prevUser.VlessUUID
	}

	dbFields, activeSquads, hasSquads := normalizeUserFields(updates)
	dbFields["updated_at"] = time.Now().UTC()
	dbFields["revision"] = gorm.Expr("revision + 1")

	err := s.db.Transaction(func(tx *gorm.DB) error {
		if len(dbFields) > 0 {
			if err := tx.Model(&database.User{}).Where("id = ?", id).Updates(dbFields).Error; err != nil {
				return err
			}
		}
		if hasSquads {
			if err := tx.Where("user_id = ?", id).Delete(&database.InternalSquadMember{}).Error; err != nil {
				return err
			}
			for _, squadUUID := range activeSquads {
				squadUUID = strings.TrimSpace(squadUUID)
				if squadUUID != "" {
					member := database.InternalSquadMember{
						InternalSquadUUID: squadUUID,
						UserID:            id,
					}
					if err := tx.Create(&member).Error; err != nil {
						return err
					}
				}
			}
		}
		var current database.User
		if err := tx.Where("id = ?", id).First(&current).Error; err != nil {
			return err
		}
		if err := emitUser(tx, &current, "user.modified"); err != nil {
			return err
		}
		return jobs.EnqueueUser(tx, &current, prevVlessUUID, false)
	})

	if err != nil {
		return nil, err
	}
	updatedUser, err := s.GetByID(id)
	if err == nil && s.syncer != nil {
		if strings.ToUpper(updatedUser.Status) == "ACTIVE" {
			s.syncer.SyncUserToNodes(updatedUser, prevVlessUUID)
		} else {
			s.syncer.RemoveUserFromNodes(updatedUser)
		}
	}
	return updatedUser, err
}

func (s *Service) Resolve(id *uint64, shortUUID, username string) (*database.User, error) {
	query := s.db.Select("id", "short_uuid", "username")
	if id != nil {
		query = query.Where("id = ?", *id)
	}
	if shortUUID != "" {
		query = query.Where("short_uuid = ?", shortUUID)
	}
	if username != "" {
		query = query.Where("username = ?", username)
	}
	var user database.User
	if err := query.First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

type AccessibleNodeRow struct {
	NodeUUID          string
	NodeName          string
	CountryCode       string
	ConfigProfileUUID string
	ConfigProfileName string
	SquadUUID         string
	SquadName         string
	InboundTag        string
}

func (s *Service) AccessibleNodes(userID uint64) ([]AccessibleNodeRow, error) {
	var user database.User
	if err := s.db.Select("id").Where("id = ?", userID).First(&user).Error; err != nil {
		return nil, err
	}
	var rows []AccessibleNodeRow
	err := s.db.Table("nodes AS n").
		Joins("JOIN config_profiles AS cp ON n.active_config_profile_uuid = cp.uuid").
		Joins("JOIN config_profile_inbounds AS cpi ON cpi.profile_uuid = cp.uuid").
		Joins("JOIN config_profile_inbounds_to_nodes AS cpin ON cpin.config_profile_inbound_uuid = cpi.uuid AND cpin.node_uuid = n.uuid").
		Joins("JOIN internal_squad_inbounds AS isi ON isi.inbound_uuid = cpi.uuid").
		Joins("JOIN internal_squads AS sq ON sq.uuid = isi.internal_squad_uuid").
		Joins("JOIN internal_squad_members AS ism ON ism.internal_squad_uuid = sq.uuid AND ism.user_id = ?", userID).
		Select("n.uuid AS node_uuid, n.name AS node_name, n.country_code, cp.uuid AS config_profile_uuid, cp.name AS config_profile_name, sq.uuid AS squad_uuid, sq.name AS squad_name, cpi.tag AS inbound_tag").
		Order("n.view_position ASC").Scan(&rows).Error
	return rows, err
}
