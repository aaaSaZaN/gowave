package squads

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"gowave/internal/configprofiles"
	"gowave/internal/database"
	"gowave/internal/jobs"
)

type Handler struct {
	db *gorm.DB
}

func NewHandler(db *gorm.DB) *Handler {
	return &Handler{db: db}
}

func uniqueInboundUUIDs(inboundUUIDs []string) []string {
	seen := make(map[string]struct{}, len(inboundUUIDs))
	result := make([]string, 0, len(inboundUUIDs))
	for _, inboundUUID := range inboundUUIDs {
		inboundUUID = strings.TrimSpace(inboundUUID)
		if inboundUUID == "" {
			continue
		}
		if _, exists := seen[inboundUUID]; exists {
			continue
		}
		seen[inboundUUID] = struct{}{}
		result = append(result, inboundUUID)
	}
	return result
}

func internalSquadTags(raw database.StringArray) []string {
	if raw == nil {
		return []string{}
	}
	return []string(raw)
}

func formatInternalSquad(db *gorm.DB, s *database.InternalSquad) map[string]interface{} {
	tags := internalSquadTags(s.Tags)

	var membersCount int64
	db.Model(&database.InternalSquadMember{}).Where("internal_squad_uuid = ?", s.UUID).Count(&membersCount)

	var inboundsCount int64
	db.Model(&database.InternalSquadInbound{}).Where("internal_squad_uuid = ?", s.UUID).Count(&inboundsCount)

	var cpiList []database.ConfigProfileInbound
	db.Table("config_profile_inbounds").
		Joins("JOIN internal_squad_inbounds ON internal_squad_inbounds.inbound_uuid = config_profile_inbounds.uuid").
		Where("internal_squad_inbounds.internal_squad_uuid = ?", s.UUID).
		Find(&cpiList)

	inboundsRes := make([]map[string]interface{}, 0, len(cpiList))
	for _, inb := range cpiList {
		var rawInb json.RawMessage = []byte("{}")
		if inb.RawInbound != "" && inb.RawInbound != "null" {
			if sorted, err := configprofiles.SortInboundConfig([]byte(inb.RawInbound)); err == nil {
				rawInb = json.RawMessage(sorted)
			} else {
				rawInb = json.RawMessage(inb.RawInbound)
			}
		}
		inboundsRes = append(inboundsRes, map[string]interface{}{
			"uuid":        inb.UUID,
			"profileUuid": inb.ProfileUUID,
			"tag":         inb.Tag,
			"type":        inb.Type,
			"network":     inb.Network,
			"security":    inb.Security,
			"port":        inb.Port,
			"rawInbound":  rawInb,
		})
	}

	return map[string]interface{}{
		"uuid":         s.UUID,
		"viewPosition": s.ViewPosition,
		"name":         s.Name,
		"tags":         tags,
		"info": map[string]interface{}{
			"membersCount":  membersCount,
			"inboundsCount": inboundsCount,
		},
		"inbounds":  inboundsRes,
		"createdAt": s.CreatedAt.UTC().Format(time.RFC3339),
		"updatedAt": s.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func (h *Handler) GetInternalSquads(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var squads []database.InternalSquad
	h.db.Order("view_position asc, created_at asc").Find(&squads)

	res := make([]map[string]interface{}, 0, len(squads))
	for _, s := range squads {
		res = append(res, formatInternalSquad(h.db, &s))
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": map[string]interface{}{
			"total":          len(res),
			"internalSquads": res,
		},
	})
}

func (h *Handler) GetInternalSquad(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	uuidParam := chi.URLParam(r, "uuid")

	var s database.InternalSquad
	if err := h.db.Where("uuid = ?", uuidParam).First(&s).Error; err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Squad not found"})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": formatInternalSquad(h.db, &s),
	})
}

func (h *Handler) CreateInternalSquad(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var body struct {
		Name         string   `json:"name"`
		Inbounds     []string `json:"inbounds"`
		InboundUUIDs []string `json:"inboundUuids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Invalid body"})
		return
	}

	inboundIDs := body.Inbounds
	if len(inboundIDs) == 0 && len(body.InboundUUIDs) > 0 {
		inboundIDs = body.InboundUUIDs
	}
	inboundIDs = uniqueInboundUUIDs(inboundIDs)

	var count int64
	h.db.Model(&database.InternalSquad{}).Count(&count)

	newUUID := uuid.New().String()
	now := time.Now().UTC()
	s := database.InternalSquad{
		UUID:         newUUID,
		ViewPosition: int(count) + 1,
		Name:         strings.TrimSpace(body.Name),
		Tags:         database.StringArray{},
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	err := h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Table("internal_squads").Create(map[string]interface{}{
			"uuid":          s.UUID,
			"view_position": s.ViewPosition,
			"name":          s.Name,
			"tags":          database.StringArray{},
			"created_at":    s.CreatedAt,
			"updated_at":    s.UpdatedAt,
		}).Error; err != nil {
			return err
		}

		links := make([]database.InternalSquadInbound, 0, len(inboundIDs))
		for _, inbID := range inboundIDs {
			links = append(links, database.InternalSquadInbound{
				InternalSquadUUID: newUUID,
				InboundUUID:       inbID,
			})
		}
		if len(links) == 0 {
			return nil
		}
		return tx.Create(&links).Error
	})
	if err != nil {
		log.Printf("[SQUADS] failed to create squad name=%q: %v", s.Name, err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Failed to create squad"})
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": formatInternalSquad(h.db, &s),
	})
}

func (h *Handler) UpdateInternalSquad(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var body struct {
		UUID         string    `json:"uuid"`
		Name         *string   `json:"name"`
		Inbounds     *[]string `json:"inbounds"`
		InboundUUIDs *[]string `json:"inboundUuids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Invalid body"})
		return
	}

	var s database.InternalSquad
	if err := h.db.Where("uuid = ?", body.UUID).First(&s).Error; err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Squad not found"})
		return
	}

	if body.Name != nil {
		s.Name = strings.TrimSpace(*body.Name)
	}

	var targetInbounds *[]string
	if body.Inbounds != nil {
		targetInbounds = body.Inbounds
	} else if body.InboundUUIDs != nil {
		targetInbounds = body.InboundUUIDs
	}

	updatedAt := time.Now().UTC()
	updates := map[string]interface{}{"updated_at": updatedAt}
	if body.Name != nil {
		updates["name"] = s.Name
	}
	err := h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&database.InternalSquad{}).
			Where("uuid = ?", s.UUID).
			Updates(updates).Error; err != nil {
			return err
		}

		if targetInbounds == nil {
			return nil
		}
		if err := tx.Where("internal_squad_uuid = ?", s.UUID).
			Delete(&database.InternalSquadInbound{}).Error; err != nil {
			return err
		}

		inboundUUIDs := uniqueInboundUUIDs(*targetInbounds)
		links := make([]database.InternalSquadInbound, 0, len(inboundUUIDs))
		for _, inboundUUID := range inboundUUIDs {
			links = append(links, database.InternalSquadInbound{
				InternalSquadUUID: s.UUID,
				InboundUUID:       inboundUUID,
			})
		}
		if len(links) > 0 {
			if err := tx.Create(&links).Error; err != nil {
				return err
			}
		}
		var ids []uint64
		if err := tx.Model(&database.InternalSquadMember{}).Where("internal_squad_uuid=?", s.UUID).Pluck("user_id", &ids).Error; err != nil {
			return err
		}
		return enqueueAffected(tx, ids)
	})
	if err != nil {
		log.Printf("[SQUADS] failed to update squad uuid=%s: %v", s.UUID, err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Failed to update squad"})
		return
	}
	s.UpdatedAt = updatedAt

	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": formatInternalSquad(h.db, &s),
	})
}

func (h *Handler) DeleteInternalSquad(w http.ResponseWriter, r *http.Request) {
	uuidParam := chi.URLParam(r, "uuid")
	if err := h.db.Transaction(func(tx *gorm.DB) error {
		var ids []uint64
		if err := tx.Model(&database.InternalSquadMember{}).Where("internal_squad_uuid=?", uuidParam).Pluck("user_id", &ids).Error; err != nil {
			return err
		}
		if err := tx.Where("squad_uuid = ?", uuidParam).Delete(&database.InternalSquadHostLink{}).Error; err != nil {
			return err
		}
		if err := tx.Where("internal_squad_uuid = ?", uuidParam).Delete(&database.InternalSquadInbound{}).Error; err != nil {
			return err
		}
		if err := tx.Where("internal_squad_uuid = ?", uuidParam).Delete(&database.InternalSquadMember{}).Error; err != nil {
			return err
		}
		if err := tx.Where("uuid = ?", uuidParam).Delete(&database.InternalSquad{}).Error; err != nil {
			return err
		}
		return enqueueAffected(tx, ids)
	}); err != nil {
		log.Printf("[SQUADS] failed to delete squad uuid=%s: %v", uuidParam, err)
		http.Error(w, `{"message":"Failed to delete squad"}`, http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ReorderInternalSquads(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var body struct {
		Items []struct {
			UUID         string `json:"uuid"`
			ViewPosition int    `json:"viewPosition"`
		} `json:"items"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Invalid body"})
		return
	}

	for _, item := range body.Items {
		h.db.Model(&database.InternalSquad{}).Where("uuid = ?", item.UUID).Update("view_position", item.ViewPosition)
	}

	var squads []database.InternalSquad
	h.db.Order("view_position asc, created_at asc").Find(&squads)
	res := make([]map[string]interface{}, 0, len(squads))
	for _, s := range squads {
		res = append(res, formatInternalSquad(h.db, &s))
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": map[string]interface{}{
			"total":          len(res),
			"internalSquads": res,
		},
	})
}

func (h *Handler) GetInternalSquadsTags(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var squads []database.InternalSquad
	h.db.Find(&squads)

	tagMap := make(map[string]bool)
	for _, s := range squads {
		for _, tag := range internalSquadTags(s.Tags) {
			tagMap[tag] = true
		}
	}

	tagList := make([]string, 0, len(tagMap))
	for tag := range tagMap {
		tagList = append(tagList, tag)
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": map[string]interface{}{
			"tags": tagList,
		},
	})
}

func (h *Handler) SetInternalSquadsTags(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var body struct {
		UUID string   `json:"uuid"`
		Tags []string `json:"tags"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Invalid body"})
		return
	}

	if body.Tags == nil {
		body.Tags = []string{}
	}
	h.db.Model(&database.InternalSquad{}).Where("uuid = ?", body.UUID).Update("tags", database.StringArray(body.Tags))

	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": map[string]interface{}{
			"uuid": body.UUID,
			"tags": body.Tags,
		},
	})
}

type accessibleNodeRow struct {
	NodeUUID          string `gorm:"column:node_uuid"`
	NodeName          string `gorm:"column:node_name"`
	CountryCode       string `gorm:"column:country_code"`
	ViewPosition      int    `gorm:"column:view_position"`
	ConfigProfileUUID string `gorm:"column:config_profile_uuid"`
	ConfigProfileName string `gorm:"column:config_profile_name"`
	InboundTag        string `gorm:"column:inbound_tag"`
}

func (h *Handler) GetInternalSquadAccessibleNodes(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	uuidParam := chi.URLParam(r, "uuid")

	var rows []accessibleNodeRow
	h.db.Raw(`
		SELECT 
			n.uuid as node_uuid,
			n.name as node_name,
			n.country_code as country_code,
			n.view_position as view_position,
			cp.uuid as config_profile_uuid,
			cp.name as config_profile_name,
			cpi.tag as inbound_tag
		FROM nodes n
		INNER JOIN config_profiles cp ON n.active_config_profile_uuid = cp.uuid
		INNER JOIN config_profile_inbounds cpi ON cpi.profile_uuid = cp.uuid
		INNER JOIN config_profile_inbounds_to_nodes cpin ON cpin.config_profile_inbound_uuid = cpi.uuid AND cpin.node_uuid = n.uuid
		INNER JOIN internal_squad_inbounds isi ON isi.inbound_uuid = cpi.uuid
		WHERE isi.internal_squad_uuid = ?
		ORDER BY n.view_position ASC, n.created_at ASC
	`, uuidParam).Scan(&rows)

	type nodeInfo struct {
		UUID              string   `json:"uuid"`
		NodeName          string   `json:"nodeName"`
		CountryCode       string   `json:"countryCode"`
		ConfigProfileUUID string   `json:"configProfileUuid"`
		ConfigProfileName string   `json:"configProfileName"`
		ActiveInbounds    []string `json:"activeInbounds"`
	}

	orderedKeys := make([]string, 0)
	nodeMap := make(map[string]*nodeInfo)

	for _, r := range rows {
		if _, exists := nodeMap[r.NodeUUID]; !exists {
			orderedKeys = append(orderedKeys, r.NodeUUID)
			nodeMap[r.NodeUUID] = &nodeInfo{
				UUID:              r.NodeUUID,
				NodeName:          r.NodeName,
				CountryCode:       r.CountryCode,
				ConfigProfileUUID: r.ConfigProfileUUID,
				ConfigProfileName: r.ConfigProfileName,
				ActiveInbounds:    []string{},
			}
		}
		item := nodeMap[r.NodeUUID]
		item.ActiveInbounds = append(item.ActiveInbounds, r.InboundTag)
	}

	nodesList := make([]*nodeInfo, 0, len(orderedKeys))
	for _, key := range orderedKeys {
		nodesList = append(nodesList, nodeMap[key])
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": map[string]interface{}{
			"squadUuid":       uuidParam,
			"accessibleNodes": nodesList,
		},
	})
}

func enqueueAffected(tx *gorm.DB, ids []uint64) error {
	for _, id := range ids {
		if err := tx.Model(&database.User{}).Where("id=?", id).Updates(map[string]interface{}{"revision": gorm.Expr("revision + 1"), "updated_at": time.Now().UTC()}).Error; err != nil {
			return err
		}
		var user database.User
		if err := tx.First(&user, id).Error; err != nil {
			return err
		}
		if err := jobs.EnqueueUser(tx, &user, nil, false); err != nil {
			return err
		}
	}
	return nil
}
func (h *Handler) SquadBulkAction(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "uuid")
	internal := strings.Contains(r.URL.Path, "/internal-squads/")
	action := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
	if action != "add-users" && action != "remove-users" && action != "add-many-users" && action != "remove-many-users" {
		http.Error(w, "unknown action", 400)
		return
	}
	var ids []uint64
	if strings.Contains(action, "many") {
		var body struct {
			IDs []uint64 `json:"userIds"`
		}
		if json.NewDecoder(r.Body).Decode(&body) != nil {
			http.Error(w, "invalid body", 400)
			return
		}
		ids = body.IDs
	}
	err := h.db.Transaction(func(tx *gorm.DB) error {
		if internal {
			var squad database.InternalSquad
			if err := tx.Where("uuid=?", id).First(&squad).Error; err != nil {
				return err
			}
		} else {
			var squad database.ExternalSquad
			if err := tx.Where("uuid=?", id).First(&squad).Error; err != nil {
				return err
			}
		}
		if !strings.Contains(action, "many") {
			query := tx.Model(&database.User{})
			if strings.HasPrefix(action, "remove") {
				if internal {
					query = query.Where("id IN (SELECT user_id FROM internal_squad_members WHERE internal_squad_uuid=?)", id)
				} else {
					query = query.Where("external_squad_uuid=?", id)
				}
			}
			if err := query.Pluck("id", &ids).Error; err != nil {
				return err
			}
		}
		for _, userID := range ids {
			var user database.User
			if err := tx.First(&user, userID).Error; err != nil {
				return err
			}
			if internal {
				if strings.HasPrefix(action, "add") {
					if err := tx.Exec("INSERT INTO internal_squad_members (internal_squad_uuid,user_id) VALUES (?,?) ON CONFLICT DO NOTHING", id, userID).Error; err != nil {
						return err
					}
				} else {
					if err := tx.Where("internal_squad_uuid=? AND user_id=?", id, userID).Delete(&database.InternalSquadMember{}).Error; err != nil {
						return err
					}
				}
			} else {
				var target interface{} = id
				query := tx.Model(&user)
				if strings.HasPrefix(action, "remove") {
					target = nil
					query = query.Where("external_squad_uuid = ?", id)
				}
				if err := query.Update("external_squad_uuid", target).Error; err != nil {
					return err
				}
			}
		}
		return enqueueAffected(tx, ids)
	})
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"response": map[string]bool{"success": true}})
}

// nullableJSON decodes an optional JSON column. Empty strings, null and the
// column default "{}" all mean "not configured", which upstream reports as null.
func nullableJSON(raw string) interface{} {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" || raw == "{}" {
		return nil
	}
	var value interface{}
	if json.Unmarshal([]byte(raw), &value) != nil {
		return nil
	}
	return value
}

func formatExternalSquad(db *gorm.DB, s *database.ExternalSquad) map[string]interface{} {
	tags := []string(s.Tags)

	subSettings := nullableJSON(s.SubscriptionSettings)
	hostOverrides := nullableJSON(s.HostOverrides)
	respHeadersAdd := nullableJSON(s.ResponseHeadersAdd)
	if respHeadersAdd == nil {
		respHeadersAdd = map[string]interface{}{}
	}
	respHeadersRemove := []string(s.ResponseHeadersRemove)
	if respHeadersRemove == nil {
		respHeadersRemove = []string{}
	}
	if tags == nil {
		tags = []string{}
	}
	hwidSettings := nullableJSON(s.HwidSettings)
	customRemarks := nullableJSON(s.CustomRemarks)

	var membersCount int64
	db.Model(&database.User{}).Where("external_squad_uuid = ?", s.UUID).Count(&membersCount)

	var tmpls []database.ExternalSquadTemplate
	db.Where("external_squad_uuid = ?", s.UUID).Find(&tmpls)
	templatesRes := make([]map[string]interface{}, 0, len(tmpls))
	for _, t := range tmpls {
		templatesRes = append(templatesRes, map[string]interface{}{
			"templateUuid": t.TemplateUUID,
			"templateType": t.TemplateType,
		})
	}

	return map[string]interface{}{
		"uuid":         s.UUID,
		"viewPosition": s.ViewPosition,
		"name":         s.Name,
		"tags":         tags,
		"info": map[string]interface{}{
			"membersCount": membersCount,
		},
		"templates":             templatesRes,
		"subscriptionSettings":  subSettings,
		"hostOverrides":         hostOverrides,
		"responseHeadersAdd":    respHeadersAdd,
		"responseHeadersRemove": respHeadersRemove,
		"hwidSettings":          hwidSettings,
		"customRemarks":         customRemarks,
		"subpageConfigUuid":     s.SubpageConfigUUID,
		"createdAt":             s.CreatedAt.UTC().Format(time.RFC3339),
		"updatedAt":             s.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func (h *Handler) GetExternalSquads(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var squads []database.ExternalSquad
	h.db.Order("view_position asc, created_at asc").Find(&squads)

	res := make([]map[string]interface{}, 0, len(squads))
	for _, s := range squads {
		res = append(res, formatExternalSquad(h.db, &s))
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": map[string]interface{}{
			"total":          len(res),
			"externalSquads": res,
		},
	})
}

func (h *Handler) GetExternalSquad(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	uuidParam := chi.URLParam(r, "uuid")

	var s database.ExternalSquad
	if err := h.db.Where("uuid = ?", uuidParam).First(&s).Error; err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Squad not found"})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": formatExternalSquad(h.db, &s),
	})
}

func (h *Handler) CreateExternalSquad(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var body struct {
		Name      string `json:"name"`
		Templates []struct {
			TemplateUUID string `json:"templateUuid"`
			TemplateType string `json:"templateType"`
		} `json:"templates"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Invalid body"})
		return
	}

	var count int64
	h.db.Model(&database.ExternalSquad{}).Count(&count)

	newUUID := uuid.New().String()
	now := time.Now().UTC()

	s := database.ExternalSquad{
		UUID:                  newUUID,
		ViewPosition:          int(count) + 1,
		Name:                  strings.TrimSpace(body.Name),
		Tags:                  database.StringArray{},
		SubscriptionSettings:  "{}",
		HostOverrides:         "{}",
		ResponseHeadersAdd:    "{}",
		ResponseHeadersRemove: database.StringArray{},
		CreatedAt:             now,
		UpdatedAt:             now,
	}

	if err := h.db.Create(&s).Error; err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Failed to create squad"})
		return
	}

	for _, tmpl := range body.Templates {
		h.db.Create(&database.ExternalSquadTemplate{
			ExternalSquadUUID: newUUID,
			TemplateType:      tmpl.TemplateType,
			TemplateUUID:      tmpl.TemplateUUID,
		})
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": formatExternalSquad(h.db, &s),
	})
}

func (h *Handler) UpdateExternalSquad(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	requestBody, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Invalid body"})
		return
	}
	var rawFields map[string]json.RawMessage
	if err := json.Unmarshal(requestBody, &rawFields); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Invalid body"})
		return
	}

	var body struct {
		UUID      string  `json:"uuid"`
		Name      *string `json:"name"`
		Templates *[]struct {
			TemplateUUID string `json:"templateUuid"`
			TemplateType string `json:"templateType"`
		} `json:"templates"`
		SubpageConfigUUID *string `json:"subpageConfigUuid"`
	}
	if err := json.Unmarshal(requestBody, &body); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Invalid body"})
		return
	}

	var s database.ExternalSquad
	if err := h.db.Where("uuid = ?", body.UUID).First(&s).Error; err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Squad not found"})
		return
	}

	updates := map[string]interface{}{"updated_at": time.Now().UTC()}
	if body.Name != nil {
		updates["name"] = strings.TrimSpace(*body.Name)
	}
	for field, column := range map[string]string{
		"subscriptionSettings": "subscription_settings",
		"hostOverrides":        "host_overrides",
		"responseHeadersAdd":   "response_headers_add",
		"hwidSettings":         "hwid_settings",
		"customRemarks":        "custom_remarks",
	} {
		rawValue, present := rawFields[field]
		if !present {
			continue
		}
		if bytes.Equal(bytes.TrimSpace(rawValue), []byte("null")) {
			updates[column] = nil
			continue
		}
		var compact bytes.Buffer
		if err := json.Compact(&compact, rawValue); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{"message": "Invalid body"})
			return
		}
		if h.db.Dialector.Name() == "postgres" {
			updates[column] = gorm.Expr("?::jsonb", compact.String())
		} else {
			updates[column] = compact.String()
		}
	}
	if rawValue, present := rawFields["responseHeadersRemove"]; present {
		var responseHeadersRemove []string
		if err := json.Unmarshal(rawValue, &responseHeadersRemove); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{"message": "Invalid body"})
			return
		}
		updates["response_headers_remove"] = database.StringArray(responseHeadersRemove)
	}
	if rawValue, present := rawFields["subpageConfigUuid"]; present {
		if bytes.Equal(bytes.TrimSpace(rawValue), []byte("null")) {
			updates["subpage_config_uuid"] = nil
		} else if body.SubpageConfigUUID != nil {
			updates["subpage_config_uuid"] = *body.SubpageConfigUUID
		}
	}

	err = h.db.Transaction(func(tx *gorm.DB) error {
		if body.Templates != nil {
			if err := tx.Where("external_squad_uuid = ?", s.UUID).Delete(&database.ExternalSquadTemplate{}).Error; err != nil {
				return err
			}
			for _, tmpl := range *body.Templates {
				if err := tx.Create(&database.ExternalSquadTemplate{
					ExternalSquadUUID: s.UUID,
					TemplateType:      tmpl.TemplateType,
					TemplateUUID:      tmpl.TemplateUUID,
				}).Error; err != nil {
					return err
				}
			}
		}
		return tx.Model(&database.ExternalSquad{}).Where("uuid = ?", s.UUID).Updates(updates).Error
	})
	if err != nil {
		log.Printf("[SQUADS] failed to update external squad uuid=%s: %v", s.UUID, err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Failed to update squad"})
		return
	}
	if err := h.db.Where("uuid = ?", s.UUID).First(&s).Error; err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Failed to load updated squad"})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": formatExternalSquad(h.db, &s),
	})
}

func (h *Handler) DeleteExternalSquad(w http.ResponseWriter, r *http.Request) {
	uuidParam := chi.URLParam(r, "uuid")
	h.db.Where("external_squad_uuid = ?", uuidParam).Delete(&database.ExternalSquadTemplate{})
	h.db.Model(&database.User{}).Where("external_squad_uuid = ?", uuidParam).Update("external_squad_uuid", nil)
	h.db.Where("uuid = ?", uuidParam).Delete(&database.ExternalSquad{})
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ReorderExternalSquads(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var body struct {
		Items []struct {
			UUID         string `json:"uuid"`
			ViewPosition int    `json:"viewPosition"`
		} `json:"items"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Invalid body"})
		return
	}

	for _, item := range body.Items {
		h.db.Model(&database.ExternalSquad{}).Where("uuid = ?", item.UUID).Update("view_position", item.ViewPosition)
	}

	var squads []database.ExternalSquad
	h.db.Order("view_position asc, created_at asc").Find(&squads)
	res := make([]map[string]interface{}, 0, len(squads))
	for _, s := range squads {
		res = append(res, formatExternalSquad(h.db, &s))
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": map[string]interface{}{
			"total":          len(res),
			"externalSquads": res,
		},
	})
}

func (h *Handler) GetExternalSquadsTags(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var squads []database.ExternalSquad
	h.db.Find(&squads)

	tagMap := make(map[string]bool)
	for _, s := range squads {
		for _, tag := range s.Tags {
			tagMap[tag] = true
		}
	}

	tagList := make([]string, 0, len(tagMap))
	for tag := range tagMap {
		tagList = append(tagList, tag)
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": map[string]interface{}{
			"tags": tagList,
		},
	})
}

func (h *Handler) SetExternalSquadsTags(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var body struct {
		UUID string   `json:"uuid"`
		Tags []string `json:"tags"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Invalid body"})
		return
	}

	h.db.Model(&database.ExternalSquad{}).Where("uuid = ?", body.UUID).Update("tags", database.StringArray(body.Tags))

	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": map[string]interface{}{
			"uuid": body.UUID,
			"tags": body.Tags,
		},
	})
}
