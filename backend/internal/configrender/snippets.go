package configrender

import (
	"encoding/json"
	"fmt"
	"gorm.io/gorm"
	"gowave/internal/database"
)

// Expand matches upstream root and array snippet precedence. Missing array
// snippets are removed; root snippets cannot override protected profile keys.
func Expand(db *gorm.DB, raw string) (map[string]interface{}, error) {
	var config map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &config); err != nil || config == nil {
		return nil, fmt.Errorf("invalid Xray configuration")
	}
	var records []database.ConfigProfileSnippet
	if err := db.Find(&records).Error; err != nil {
		return nil, err
	}
	snippets := map[string]interface{}{}
	for _, record := range records {
		var value interface{}
		if err := json.Unmarshal([]byte(record.Snippet), &value); err != nil {
			return nil, fmt.Errorf("snippet %s: %w", record.Name, err)
		}
		snippets[record.Name] = value
	}
	names, _ := config["snippets"].([]interface{})
	delete(config, "snippets")
	merged := map[string]interface{}{}
	for _, name := range names {
		key, _ := name.(string)
		value := snippets[key]
		parts, ok := value.([]interface{})
		if !ok {
			parts = []interface{}{value}
		}
		for _, part := range parts {
			if object, ok := part.(map[string]interface{}); ok {
				for k, v := range object {
					merged[k] = v
				}
			}
		}
	}
	protected := map[string]bool{"api": true, "inbounds": true, "metrics": true, "snippets": true, "stats": true}
	for k, v := range merged {
		if _, exists := config[k]; !exists && !protected[k] {
			config[k] = v
		}
	}
	expandArray := func(parent map[string]interface{}, key string) {
		items, ok := parent[key].([]interface{})
		if !ok {
			return
		}
		result := make([]interface{}, 0, len(items))
		for _, item := range items {
			object, ok := item.(map[string]interface{})
			if !ok {
				result = append(result, item)
				continue
			}
			name, ok := object["snippet"].(string)
			if !ok {
				result = append(result, item)
				continue
			}
			value, ok := snippets[name]
			if !ok {
				continue
			}
			if parts, ok := value.([]interface{}); ok {
				result = append(result, parts...)
			} else {
				result = append(result, value)
			}
		}
		parent[key] = result
	}
	expandArray(config, "outbounds")
	if routing, ok := config["routing"].(map[string]interface{}); ok {
		expandArray(routing, "rules")
		expandArray(routing, "balancers")
	}
	return config, nil
}
