package nodes

import (
	"encoding/json"
	"fmt"
	"gowave/internal/database"
	"gowave/internal/jobs"
	"math"
	"reflect"
	"strings"

	"gorm.io/gorm"
)

func (s *Service) BulkUpdate(ids []string, fields map[string]interface{}) error {
	statement := &gorm.Statement{DB: s.db}
	if err := statement.Parse(&database.Node{}); err != nil {
		return err
	}
	updates := map[string]interface{}{}
	for key, value := range fields {
		if key == "consumptionMultiplier" || key == "nodeConsumptionMultiplier" {
			number, ok := value.(float64)
			if !ok || number < 0 || number > 100 || math.IsNaN(number) {
				return fmt.Errorf("invalid %s", key)
			}
			column := "consumption_multiplier"
			if key == "nodeConsumptionMultiplier" {
				column = "node_consumption_multiplier"
			}
			updates[column] = MultiplierToNano(number)
			continue
		}
		if key == "tags" || key == "integrationUuids" {
			raw, err := json.Marshal(value)
			if err != nil {
				return err
			}
			var array database.StringArray
			if err := json.Unmarshal(raw, &array); err != nil {
				return err
			}
			column := "tags"
			if key == "integrationUuids" {
				column = "integration_uuids"
			}
			updates[column] = array
			continue
		}
		found := false
		for _, field := range statement.Schema.Fields {
			if strings.Split(field.Tag.Get("json"), ",")[0] != key || field.DBName == "" {
				continue
			}
			found = true
			if field.PrimaryKey || field.Name == "CreatedAt" || field.Name == "UpdatedAt" || field.Name == "IsConnected" || field.Name == "IsConnecting" || field.Name == "IsDisabled" || field.Name == "TrafficUsedBytes" || field.Name == "ActiveConfigProfileUUID" {
				return fmt.Errorf("field %s requires a node action", key)
			}
			if value == nil {
				if field.FieldType.Kind() != reflect.Pointer {
					return fmt.Errorf("%s cannot be null", key)
				}
				updates[field.DBName] = nil
				break
			}
			raw, err := json.Marshal(value)
			if err != nil {
				return err
			}
			decoded := reflect.New(field.FieldType)
			if err := json.Unmarshal(raw, decoded.Interface()); err != nil {
				return fmt.Errorf("%s: %w", key, err)
			}
			updates[field.DBName] = decoded.Elem().Interface()
			break
		}
		if !found {
			return fmt.Errorf("unknown node field %s", key)
		}
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		for _, id := range ids {
			var node database.Node
			if err := tx.Where("uuid=?", id).First(&node).Error; err != nil {
				return err
			}
			if err := tx.Model(&node).Updates(updates).Error; err != nil {
				return err
			}
			if err := jobs.EnqueueNode(tx, id, "node-start"); err != nil {
				return err
			}
		}
		return nil
	})
}
