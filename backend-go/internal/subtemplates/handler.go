package subtemplates

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"gowave/internal/database"
	"gowave/internal/httpx"
	"gowave/internal/notify"
)

type Handler struct {
	db *gorm.DB
}

func NewHandler(db *gorm.DB) *Handler {
	return &Handler{db: db}
}

func formatTemplateResponse(t *database.SubscriptionTemplate) map[string]interface{} {
	tags := []string(t.Tags)
	if tags == nil {
		tags = []string{}
	}

	var tJson json.RawMessage
	if t.TemplateJson != "" && t.TemplateJson != "null" {
		tJson = json.RawMessage(t.TemplateJson)
	}

	var yamlVal *string
	if t.TemplateYaml != "" {
		encoded := base64.StdEncoding.EncodeToString([]byte(t.TemplateYaml))
		yamlVal = &encoded
	}

	return map[string]interface{}{
		"uuid":                t.UUID,
		"viewPosition":        t.ViewPosition,
		"name":                t.Name,
		"tags":                tags,
		"templateType":        t.TemplateType,
		"templateJson":        tJson,
		"encodedTemplateYaml": yamlVal,
	}
}

func (h *Handler) GetAllTemplates(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var templates []database.SubscriptionTemplate
	h.db.Order("view_position asc, created_at asc").Find(&templates)
	if len(templates) < 5 {
		database.SeedDefaults(h.db)
		h.db.Order("view_position asc, created_at asc").Find(&templates)
	}

	res := make([]map[string]interface{}, 0, len(templates))
	for _, t := range templates {
		res = append(res, formatTemplateResponse(&t))
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": map[string]interface{}{
			"total":     len(res),
			"templates": res,
		},
	})
}

func (h *Handler) GetTemplateByUuid(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	uuidParam := chi.URLParam(r, "uuid")

	var t database.SubscriptionTemplate
	if err := h.db.Where("uuid = ?", uuidParam).First(&t).Error; err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message": "Template not found",
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": formatTemplateResponse(&t),
	})
}

func (h *Handler) CreateTemplate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var body struct {
		Name         string `json:"name"`
		TemplateType string `json:"templateType"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Invalid body"})
		return
	}

	var count int64
	h.db.Model(&database.SubscriptionTemplate{}).Count(&count)

	newUUID := uuid.New().String()
	now := time.Now().UTC()

	var defaultJson string
	var defaultYaml string

	switch body.TemplateType {
	case "XRAY_JSON":
		defaultJson = database.DefaultTemplateXrayJson
	case "SINGBOX":
		defaultJson = database.DefaultTemplateSingbox
	case "MIHOMO":
		defaultYaml = database.DefaultTemplateMihomo
	case "CLASH":
		defaultYaml = database.DefaultTemplateClash
	case "STASH":
		defaultYaml = database.DefaultTemplateStash
	default:
		defaultYaml = ""
	}

	t := database.SubscriptionTemplate{
		UUID:         newUUID,
		ViewPosition: int(count) + 1,
		Name:         strings.TrimSpace(body.Name),
		Tags:         database.StringArray{},
		TemplateType: body.TemplateType,
		TemplateYaml: defaultYaml,
		TemplateJson: defaultJson,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := h.db.Create(&t).Error; err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Failed to create template"})
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": formatTemplateResponse(&t),
	})
}

func (h *Handler) UpdateTemplate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var body struct {
		UUID                string          `json:"uuid"`
		Name                *string         `json:"name"`
		TemplateJson        json.RawMessage `json:"templateJson"`
		EncodedTemplateYaml *string         `json:"encodedTemplateYaml"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Invalid body"})
		return
	}

	var t database.SubscriptionTemplate
	if err := h.db.Where("uuid = ?", body.UUID).First(&t).Error; err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Template not found"})
		return
	}

	if body.Name != nil {
		t.Name = strings.TrimSpace(*body.Name)
	}
	if len(body.TemplateJson) > 0 && string(body.TemplateJson) != "null" {
		t.TemplateJson = string(body.TemplateJson)
	}
	if body.EncodedTemplateYaml != nil {
		decoded, err := base64.StdEncoding.DecodeString(*body.EncodedTemplateYaml)
		if err == nil {
			t.TemplateYaml = string(decoded)
		} else {
			t.TemplateYaml = *body.EncodedTemplateYaml
		}
	}
	t.UpdatedAt = time.Now().UTC()

	if err := h.db.Save(&t).Error; err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Failed to update template"})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": formatTemplateResponse(&t),
	})
}

func (h *Handler) DeleteTemplate(w http.ResponseWriter, r *http.Request) {
	uuidParam := chi.URLParam(r, "uuid")
	h.db.Where("uuid = ?", uuidParam).Delete(&database.SubscriptionTemplate{})
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ReorderTemplates(w http.ResponseWriter, r *http.Request) {
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
		h.db.Model(&database.SubscriptionTemplate{}).Where("uuid = ?", item.UUID).Update("view_position", item.ViewPosition)
	}

	var templates []database.SubscriptionTemplate
	h.db.Order("view_position asc, created_at asc").Find(&templates)
	res := make([]map[string]interface{}, 0, len(templates))
	for _, t := range templates {
		res = append(res, formatTemplateResponse(&t))
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": map[string]interface{}{
			"total":     len(res),
			"templates": res,
		},
	})
}

func (h *Handler) GetTemplateTags(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var templates []database.SubscriptionTemplate
	h.db.Find(&templates)

	tagMap := make(map[string]bool)
	for _, t := range templates {
		for _, tag := range t.Tags {
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

func (h *Handler) SetTemplateTags(w http.ResponseWriter, r *http.Request) {
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

	h.db.Model(&database.SubscriptionTemplate{}).Where("uuid = ?", body.UUID).Update("tags", database.StringArray(body.Tags))

	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": map[string]interface{}{
			"uuid": body.UUID,
			"tags": body.Tags,
		},
	})
}

func formatSubpageConfigResponse(c *database.SubscriptionPageConfig) map[string]interface{} {
	tags := []string(c.Tags)
	if tags == nil {
		tags = []string{}
	}

	cfg := json.RawMessage("{}")
	if c.Config != "" && c.Config != "null" {
		cfg = json.RawMessage(c.Config)
	}

	return map[string]interface{}{
		"uuid":         c.UUID,
		"viewPosition": c.ViewPosition,
		"name":         c.Name,
		"tags":         tags,
		"config":       cfg,
	}
}

func (h *Handler) GetAllConfigs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var configs []database.SubscriptionPageConfig
	h.db.Order("view_position asc, created_at asc").Find(&configs)
	if len(configs) == 0 {
		database.SeedDefaults(h.db)
		h.db.Order("view_position asc, created_at asc").Find(&configs)
	}

	res := make([]map[string]interface{}, 0, len(configs))
	for _, c := range configs {
		res = append(res, formatSubpageConfigResponse(&c))
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": map[string]interface{}{
			"total":   len(res),
			"configs": res,
		},
	})
}

func (h *Handler) GetConfigByUuid(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	uuidParam := chi.URLParam(r, "uuid")

	var c database.SubscriptionPageConfig
	if err := h.db.Where("uuid = ?", uuidParam).First(&c).Error; err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Config not found"})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": formatSubpageConfigResponse(&c),
	})
}

func (h *Handler) CreateConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Invalid body"})
		return
	}

	var count int64
	h.db.Model(&database.SubscriptionPageConfig{}).Count(&count)

	newUUID := uuid.New().String()
	now := time.Now().UTC()

	c := database.SubscriptionPageConfig{
		UUID:         newUUID,
		ViewPosition: int(count) + 1,
		Name:         strings.TrimSpace(body.Name),
		Tags:         database.StringArray{},
		Config:       database.DefaultSubpageConfigData,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&c).Error; err != nil {
			return err
		}
		return subpageChanged(tx, "CREATED", c.UUID)
	}); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Failed to create config"})
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": formatSubpageConfigResponse(&c),
	})
}

func (h *Handler) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var body struct {
		UUID   string          `json:"uuid"`
		Name   *string         `json:"name"`
		Config json.RawMessage `json:"config"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Invalid body"})
		return
	}

	var c database.SubscriptionPageConfig
	if err := h.db.Where("uuid = ?", body.UUID).First(&c).Error; err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Config not found"})
		return
	}

	if body.Name != nil {
		c.Name = strings.TrimSpace(*body.Name)
	}
	if len(body.Config) > 0 && string(body.Config) != "null" {
		c.Config = string(body.Config)
	}
	c.UpdatedAt = time.Now().UTC()

	if err := h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&c).Error; err != nil {
			return err
		}
		return subpageChanged(tx, "UPDATED", c.UUID)
	}); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Failed to update config"})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": formatSubpageConfigResponse(&c),
	})
}

// subpageChanged emits service.subpage_config_changed like upstream.
func subpageChanged(tx *gorm.DB, action, uuid string) error {
	return notify.Emit(tx, notify.Event{Name: "service.subpage_config_changed", Data: map[string]interface{}{
		"subpageConfig": map[string]string{"action": action, "uuid": uuid},
	}})
}

const defaultSubpageConfigUUID = "00000000-0000-0000-0000-000000000000"

func (h *Handler) DeleteConfig(w http.ResponseWriter, r *http.Request) {
	uuidParam := chi.URLParam(r, "uuid")
	if uuidParam == defaultSubpageConfigUUID {
		httpx.Error(w, r, http.StatusBadRequest, "A212", "Reserved subscription page config can't be deleted")
		return
	}
	err := h.db.Transaction(func(tx *gorm.DB) error {
		result := tx.Where("uuid = ?", uuidParam).Delete(&database.SubscriptionPageConfig{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return subpageChanged(tx, "DELETED", uuidParam)
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		httpx.Error(w, r, http.StatusNotFound, "A206", "Subscription page config not found")
		return
	}
	if err != nil {
		httpx.Error(w, r, http.StatusInternalServerError, "A213", "Delete subscription page config error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) CloneConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	var body struct {
		CloneFromUUID string `json:"cloneFromUuid"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Invalid body"})
		return
	}

	var orig database.SubscriptionPageConfig
	if err := h.db.Where("uuid = ?", body.CloneFromUUID).First(&orig).Error; err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Original config not found"})
		return
	}

	var count int64
	h.db.Model(&database.SubscriptionPageConfig{}).Count(&count)

	now := time.Now().UTC()
	clone := database.SubscriptionPageConfig{
		UUID:         uuid.New().String(),
		ViewPosition: int(count) + 1,
		Name:         orig.Name + " (Clone)",
		Tags:         orig.Tags,
		Config:       orig.Config,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := h.db.Create(&clone).Error; err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Failed to clone config"})
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": formatSubpageConfigResponse(&clone),
	})
}

func (h *Handler) ReorderConfigs(w http.ResponseWriter, r *http.Request) {
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
		h.db.Model(&database.SubscriptionPageConfig{}).Where("uuid = ?", item.UUID).Update("view_position", item.ViewPosition)
	}

	var configs []database.SubscriptionPageConfig
	h.db.Order("view_position asc, created_at asc").Find(&configs)
	res := make([]map[string]interface{}, 0, len(configs))
	for _, c := range configs {
		res = append(res, formatSubpageConfigResponse(&c))
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": map[string]interface{}{
			"total":   len(res),
			"configs": res,
		},
	})
}

func (h *Handler) GetConfigTags(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var configs []database.SubscriptionPageConfig
	h.db.Find(&configs)

	tagMap := make(map[string]bool)
	for _, c := range configs {
		for _, tag := range c.Tags {
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

func (h *Handler) SetConfigTags(w http.ResponseWriter, r *http.Request) {
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

	h.db.Model(&database.SubscriptionPageConfig{}).Where("uuid = ?", body.UUID).Update("tags", database.StringArray(body.Tags))

	json.NewEncoder(w).Encode(map[string]interface{}{
		"response": map[string]interface{}{
			"uuid": body.UUID,
			"tags": body.Tags,
		},
	})
}
