package configprofiles

import (
	"encoding/json"
	"net/http"
	"remnawave-go/internal/database"
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

	// Need to decode everything into a map to know which fields to update
	var rawBody map[string]interface{}

	// Body needs to be read into bytes first so we can parse it twice
	// Or we can just use rawBody directly
	if err := json.NewDecoder(r.Body).Decode(&rawBody); err == nil {
		if uuidsInter, ok := rawBody["uuids"].([]interface{}); ok && len(uuidsInter) > 0 {
			var uuids []string
			for _, v := range uuidsInter {
				if s, ok := v.(string); ok {
					uuids = append(uuids, s)
				}
			}

			delete(rawBody, "uuids")
			if len(rawBody) > 0 && len(uuids) > 0 {
				// Map JSON fields to DB columns
				updates := make(map[string]interface{})

				// Handle some common fields
				if v, ok := rawBody["remark"]; ok {
					updates["remark"] = v
				}
				if v, ok := rawBody["address"]; ok {
					updates["address"] = v
				}
				if v, ok := rawBody["port"]; ok {
					updates["port"] = v
				}
				if v, ok := rawBody["path"]; ok {
					updates["path"] = v
				}
				if v, ok := rawBody["sni"]; ok {
					updates["sni"] = v
				}
				if v, ok := rawBody["host"]; ok {
					updates["host"] = v
				}
				if v, ok := rawBody["alpn"]; ok {
					updates["alpn"] = v
				}
				if v, ok := rawBody["fingerprint"]; ok {
					updates["fingerprint"] = v
				}
				if v, ok := rawBody["network"]; ok {
					updates["network"] = v
				}
				if v, ok := rawBody["securityLayer"]; ok {
					updates["security_layer"] = v
				}
				if v, ok := rawBody["xrayCoreTemplate"]; ok {
					updates["xray_core_template"] = v
				}
				if v, ok := rawBody["isFallback"]; ok {
					updates["is_fallback"] = v
				}
				if v, ok := rawBody["isDisabled"]; ok {
					updates["is_disabled"] = v
				}
				if v, ok := rawBody["isHidden"]; ok {
					updates["is_hidden"] = v
				}

				if len(updates) > 0 {
					h.service.DB().Model(&database.Host{}).Where("uuid IN ?", uuids).Updates(updates)
				}
			}
		}
	}

	json.NewEncoder(w).Encode(map[string]interface{}{"response": map[string]interface{}{"success": true}})
}
