package configprofiles

import (
	"encoding/json"
	"gorm.io/gorm"
	"gowave/internal/database"
	"net/http"
)

func (h *Handler) BulkDeleteHosts(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var body struct {
		UUIDs []string `json:"uuids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err == nil && len(body.UUIDs) > 0 {
		for _, uuid := range body.UUIDs {
			_ = h.service.DeleteHost(uuid)
		}
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"response": map[string]interface{}{"success": true}})
}

func (h *Handler) BulkDisableHosts(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var body struct {
		UUIDs []string `json:"uuids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err == nil && len(body.UUIDs) > 0 {
		h.service.DB().Model(&database.Host{}).Where("uuid IN ?", body.UUIDs).Update("is_disabled", true)
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"response": map[string]interface{}{"success": true}})
}

func (h *Handler) BulkEnableHosts(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var body struct {
		UUIDs []string `json:"uuids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err == nil && len(body.UUIDs) > 0 {
		h.service.DB().Model(&database.Host{}).Where("uuid IN ?", body.UUIDs).Update("is_disabled", false)
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"response": map[string]interface{}{"success": true}})
}

func (h *Handler) BulkUpdateHosts(w http.ResponseWriter, r *http.Request) {
	var patch map[string]json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		http.Error(w, "invalid body", 400)
		return
	}
	var ids []string
	if err := json.Unmarshal(patch["uuids"], &ids); err != nil || len(ids) == 0 {
		http.Error(w, "uuids required", 400)
		return
	}
	delete(patch, "uuids")
	if err := h.service.DB().Transaction(func(tx *gorm.DB) error {
		for _, id := range ids {
			if _, err := patchHost(tx, id, patch); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"response": map[string]interface{}{"success": true}})
}
