package configprofiles

import (
	"encoding/json"
	"fmt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gowave/internal/database"
	"reflect"
	"strings"
)

// patchHost decodes present fields separately so omitted, null, empty arrays
// and false have distinct meanings for both single and bulk edits.
func patchHost(tx *gorm.DB, id string, patch map[string]json.RawMessage) (*database.Host, error) {
	var host database.Host
	query := tx.Where("uuid = ?", id)
	if tx.Dialector.Name() == "postgres" {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	if err := query.First(&host).Error; err != nil {
		return nil, err
	}
	value := reflect.ValueOf(&host).Elem()
	typ := value.Type()
	jsonObjects := map[string]bool{"xhttpExtraParams": true, "muxParams": true, "sockoptParams": true, "finalMask": true, "mapper": true}
	for key, raw := range patch {
		if key == "uuid" || key == "nodes" || key == "internalSquads" || key == "inbound" {
			continue
		}
		found := false
		for i := 0; i < typ.NumField(); i++ {
			field := typ.Field(i)
			if strings.Split(field.Tag.Get("json"), ",")[0] != key {
				continue
			}
			found = true
			target := value.Field(i)
			if jsonObjects[key] {
				if !json.Valid(raw) {
					return nil, fmt.Errorf("invalid %s", key)
				}
				target.SetString(string(raw))
			} else if string(raw) == "null" {
				if target.Kind() == reflect.Bool || key == "port" || key == "remark" || key == "address" {
					return nil, fmt.Errorf("%s cannot be null", key)
				}
				target.Set(reflect.Zero(target.Type()))
			} else if err := json.Unmarshal(raw, target.Addr().Interface()); err != nil {
				return nil, fmt.Errorf("%s: %w", key, err)
			}
			break
		}
		if !found {
			return nil, fmt.Errorf("unknown host field %s", key)
		}
	}
	if raw, ok := patch["inbound"]; ok {
		var input struct {
			Profile *string `json:"configProfileUuid"`
			Inbound *string `json:"configProfileInboundUuid"`
		}
		if err := json.Unmarshal(raw, &input); err != nil {
			return nil, err
		}
		host.ConfigProfileUUID = input.Profile
		host.ConfigProfileInboundUUID = input.Inbound
	}
	if host.Port < 1 || host.Port > 65535 || strings.TrimSpace(host.Address) == "" || strings.TrimSpace(host.Remark) == "" {
		return nil, fmt.Errorf("invalid host address, port or remark")
	}
	var squadIDs []string
	_, changeSquads := patch["internalSquads"]
	if changeSquads {
		var input struct {
			Mode   string   `json:"mode"`
			Squads []string `json:"squads"`
		}
		if err := json.Unmarshal(patch["internalSquads"], &input); err != nil {
			return nil, err
		}
		if input.Mode != "EXCLUDE" && input.Mode != "ALLOW_ONLY" {
			return nil, fmt.Errorf("invalid squad mode")
		}
		host.InternalSquadsMode = input.Mode
		squadIDs = input.Squads
	}
	if host.InternalSquadsMode != "EXCLUDE" && host.InternalSquadsMode != "ALLOW_ONLY" {
		return nil, fmt.Errorf("invalid internal squad mode")
	}
	if err := tx.Omit(clause.Associations).Save(&host).Error; err != nil {
		return nil, err
	}
	_, changeNodes := patch["nodes"]
	if changeNodes || changeSquads {
		var nodeIDs []string
		if changeNodes {
			if err := json.Unmarshal(patch["nodes"], &nodeIDs); err != nil {
				return nil, err
			}
		} else if err := tx.Model(&database.HostsToNode{}).Where("host_uuid = ?", id).Pluck("node_uuid", &nodeIDs).Error; err != nil {
			return nil, err
		}
		if !changeSquads {
			if err := tx.Model(&database.InternalSquadHostLink{}).Where("host_uuid = ?", id).Pluck("squad_uuid", &squadIDs).Error; err != nil {
				return nil, err
			}
		}
		if err := replaceHostRelations(tx, id, nodeIDs, squadIDs); err != nil {
			return nil, err
		}
	}
	return &host, nil
}
