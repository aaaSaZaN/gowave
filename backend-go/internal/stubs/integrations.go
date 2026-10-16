package stubs

import (
	"encoding/json"
	"errors"
	"gowave/internal/database"
	"gowave/internal/jobs"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func formatIntegration(value database.Integration) map[string]interface{} {
	var config map[string]interface{}
	json.Unmarshal([]byte(value.Config), &config)
	return map[string]interface{}{"uuid": value.UUID, "name": value.Name, "description": value.Description, "config": config}
}
func (s *StubHandler) integrationList(w http.ResponseWriter, r *http.Request) {
	var rows []database.Integration
	if err := s.db.Order("created_at ASC").Find(&rows).Error; err != nil {
		http.Error(w, "integrations unavailable", 503)
		return
	}
	result := make([]map[string]interface{}, 0, len(rows))
	for _, row := range rows {
		result = append(result, formatIntegration(row))
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"response": map[string]interface{}{"total": len(result), "nodeIntegrations": result}})
}
func (s *StubHandler) integrationGet(w http.ResponseWriter, r *http.Request) {
	var row database.Integration
	if s.db.Where("uuid=?", chi.URLParam(r, "uuid")).First(&row).Error != nil {
		http.Error(w, "integration not found", 404)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"response": formatIntegration(row)})
}
func (s *StubHandler) integrationWrite(w http.ResponseWriter, r *http.Request) {
	var body map[string]json.RawMessage
	if json.NewDecoder(r.Body).Decode(&body) != nil {
		http.Error(w, "invalid body", 400)
		return
	}
	var row database.Integration
	var restart bool
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if r.Method == "PATCH" {
			var id string
			if err := json.Unmarshal(body["uuid"], &id); err != nil {
				return err
			}
			if err := tx.Where("uuid=?", id).First(&row).Error; err != nil {
				return err
			}
		} else {
			row.UUID = uuid.NewString()
		}
		if raw, ok := body["name"]; ok {
			if err := json.Unmarshal(raw, &row.Name); err != nil {
				return err
			}
		}
		row.Name = strings.TrimSpace(row.Name)
		if len(row.Name) < 2 || len(row.Name) > 30 {
			return errors.New("name must be 2 to 30 characters")
		}
		if raw, ok := body["description"]; ok {
			if err := json.Unmarshal(raw, &row.Description); err != nil {
				return err
			}
			if row.Description != nil && len(*row.Description) > 255 {
				return errors.New("description too long")
			}
		}
		if raw, ok := body["config"]; ok {
			var config map[string]interface{}
			if json.Unmarshal(raw, &config) != nil || config == nil {
				return errors.New("config must be an object")
			}
			row.Config = string(raw)
		}
		if row.Config == "" {
			return errors.New("config required")
		}
		if raw, ok := body["restartNodes"]; ok {
			if err := json.Unmarshal(raw, &restart); err != nil {
				return err
			}
		}
		if err := tx.Save(&row).Error; err != nil {
			return err
		}
		if restart {
			var nodes []database.Node
			if err := tx.Where("is_disabled=?", false).Find(&nodes).Error; err != nil {
				return err
			}
			for _, node := range nodes {
				for _, id := range node.IntegrationUUIDs {
					if id == row.UUID {
						if err := jobs.EnqueueNode(tx, node.UUID, "node-start"); err != nil {
							return err
						}
						break
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if r.Method == "POST" {
		w.WriteHeader(201)
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"response": formatIntegration(row)})
}
func (s *StubHandler) integrationDelete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "uuid")
	err := s.db.Transaction(func(tx *gorm.DB) error {
		result := tx.Delete(&database.Integration{}, "uuid=?", id)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		var records []database.Node
		if err := tx.Find(&records).Error; err != nil {
			return err
		}
		for _, node := range records {
			var ids database.StringArray
			changed := false
			for _, current := range node.IntegrationUUIDs {
				if current == id {
					changed = true
				} else {
					ids = append(ids, current)
				}
			}
			if changed {
				if err := tx.Model(&node).Update("integration_uuids", ids).Error; err != nil {
					return err
				}
				if err := jobs.EnqueueNode(tx, node.UUID, "node-start"); err != nil {
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
