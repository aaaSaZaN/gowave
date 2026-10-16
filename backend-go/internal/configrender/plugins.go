package configrender

import (
	"encoding/json"
	"gorm.io/gorm"
	"gowave/internal/database"
	"strings"
)

func Plugin(db *gorm.DB, raw string) (map[string]interface{}, error) {
	var config map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &config); err != nil {
		return nil, err
	}
	if config == nil {
		config = map[string]interface{}{}
	}
	delete(config, "sharedLists")
	references := map[string]bool{}
	var collect func(interface{})
	collect = func(value interface{}) {
		switch v := value.(type) {
		case string:
			if strings.HasPrefix(v, "ext:") {
				references[strings.TrimPrefix(v, "ext:")] = true
			}
		case []interface{}:
			for _, part := range v {
				collect(part)
			}
		case map[string]interface{}:
			for _, part := range v {
				collect(part)
			}
		}
	}
	collect(config)
	var lists []database.SharedList
	if err := db.Order("name ASC").Find(&lists).Error; err != nil {
		return nil, err
	}
	resolved := []interface{}{}
	for _, list := range lists {
		if !references[list.Name] {
			continue
		}
		var value map[string]interface{}
		if err := json.Unmarshal([]byte(list.Config), &value); err != nil {
			return nil, err
		}
		if value == nil {
			continue
		}
		value["name"] = "ext:" + list.Name
		resolved = append(resolved, value)
	}
	config["sharedLists"] = resolved
	return config, nil
}
