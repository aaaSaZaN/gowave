package stubs

import (
	"encoding/json"
	"gowave/internal/database"
	"gowave/internal/jobs"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func (s *StubHandler) pluginClone(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID string `json:"cloneFromUuid"`
	}
	if json.NewDecoder(r.Body).Decode(&body) != nil || body.ID == "" {
		http.Error(w, "clone source required", 400)
		return
	}
	var plugin database.NodePlugin
	if s.db.Where("uuid=?", body.ID).First(&plugin).Error != nil {
		http.Error(w, "plugin not found", 404)
		return
	}
	plugin.UUID = uuid.NewString()
	plugin.Name += " (Clone)"
	plugin.CreatedAt = time.Now().UTC()
	plugin.UpdatedAt = plugin.CreatedAt
	if err := s.db.Create(&plugin).Error; err != nil {
		http.Error(w, "clone failed", 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"response": formatPlugin(&plugin)})
}
func (s *StubHandler) pluginTags(w http.ResponseWriter, r *http.Request) {
	var records []database.NodePlugin
	if err := s.db.Select("tags").Find(&records).Error; err != nil {
		http.Error(w, "tags unavailable", 503)
		return
	}
	set := map[string]bool{}
	for _, record := range records {
		for _, tag := range record.Tags {
			set[tag] = true
		}
	}
	tags := make([]string, 0, len(set))
	for tag := range set {
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"response": map[string]interface{}{"tags": tags}})
}
func (s *StubHandler) pluginSetTags(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID   string               `json:"uuid"`
		Tags database.StringArray `json:"tags"`
	}
	if json.NewDecoder(r.Body).Decode(&body) != nil || body.ID == "" {
		http.Error(w, "invalid body", 400)
		return
	}
	result := s.db.Model(&database.NodePlugin{}).Where("uuid=?", body.ID).Update("tags", body.Tags)
	if result.Error != nil {
		http.Error(w, "tag update failed", 500)
		return
	}
	if result.RowsAffected == 0 {
		http.Error(w, "plugin not found", 404)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"response": map[string]interface{}{"uuid": body.ID, "tags": body.Tags}})
}
func enqueuePlugin(tx *gorm.DB, id string) error {
	var targets []string
	if err := tx.Model(&database.Node{}).Where("active_plugin_uuid=? AND is_disabled=?", id, false).Pluck("uuid", &targets).Error; err != nil {
		return err
	}
	revision := uint64(time.Now().UnixNano())
	for _, target := range targets {
		if err := jobs.Enqueue(tx, "plugin-sync", id, target, revision, nil); err != nil {
			return err
		}
	}
	return nil
}
func (s *StubHandler) sharedListSync(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	if json.NewDecoder(r.Body).Decode(&body) != nil || body.Name == "" {
		http.Error(w, "list name required", 400)
		return
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var list database.SharedList
		if err := tx.Where("name=?", body.Name).First(&list).Error; err != nil {
			return err
		}
		var plugins []database.NodePlugin
		if err := tx.Find(&plugins).Error; err != nil {
			return err
		}
		for _, plugin := range plugins {
			var value interface{}
			if json.Unmarshal([]byte(plugin.PluginConfig), &value) != nil {
				continue
			}
			var references func(interface{}) bool
			references = func(value interface{}) bool {
				switch v := value.(type) {
				case string:
					return v == "ext:"+body.Name
				case []interface{}:
					for _, part := range v {
						if references(part) {
							return true
						}
					}
				case map[string]interface{}:
					for _, part := range v {
						if references(part) {
							return true
						}
					}
				}
				return false
			}
			if references(value) {
				if err := enqueuePlugin(tx, plugin.UUID); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	w.WriteHeader(204)
}
func (s *StubHandler) snippetSync(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	if json.NewDecoder(r.Body).Decode(&body) != nil || strings.TrimSpace(body.Name) == "" {
		http.Error(w, "snippet name required", 400)
		return
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var snippet database.ConfigProfileSnippet
		if err := tx.Where("name=?", body.Name).First(&snippet).Error; err != nil {
			return err
		}
		var profiles []database.ConfigProfile
		if err := tx.Find(&profiles).Error; err != nil {
			return err
		}
		for _, profile := range profiles {
			encoded, _ := json.Marshal(body.Name)
			if !strings.Contains(profile.Config, string(encoded)) {
				continue
			}
			var ids []string
			if err := tx.Model(&database.Node{}).Where("active_config_profile_uuid=? AND is_disabled=?", profile.UUID, false).Pluck("uuid", &ids).Error; err != nil {
				return err
			}
			for _, id := range ids {
				if err := jobs.EnqueueNode(tx, id, "node-start"); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	w.WriteHeader(204)
}
