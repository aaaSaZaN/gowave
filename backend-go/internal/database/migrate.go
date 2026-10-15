package database

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"sync"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
)

func AutoMigrate(db *gorm.DB) error {
	if err := MigrateSchema(db); err != nil {
		return err
	}

	SeedDefaults(db)
	return nil
}

// MigrateSchema creates or updates the GoWave schema without inserting app data.
// Importers use this to prepare an empty destination before copying source rows.
func MigrateSchema(db *gorm.DB) error {
	if err := applyMigrations(db); err != nil {
		return err
	}
	return migrateLegacyHostRelations(db)
}

func schemaModels() []interface{} {
	return []interface{}{
		&Admin{},
		&User{},
		&UserTraffic{},
		&Node{},
		&Integration{},
		&NodesUserUsageHistory{},
		&NodesUsageHistory{},
		&Host{},
		&HostsToNode{},
		&InternalSquadHostLink{},
		&ConfigProfile{},
		&ConfigProfileInbound{},
		&ConfigProfileInboundsToNodes{},
		&RemnawaveSetting{},
		&ApiToken{},
		&SubscriptionTemplate{},
		&SubscriptionPageConfig{},
		&SubscriptionSetting{},
		&ConfigProfileSnippet{},
		&SharedList{},
		&NodePlugin{},
		&InternalSquad{},
		&InternalSquadMember{},
		&InternalSquadInbound{},
		&ExternalSquadTemplate{},
		&ExternalSquad{},
		&InfraProvider{},
		&InfraBillingNode{},
		&InfraBillingHistory{},
		&HwidDevice{},
		&UserSubscriptionRequestHistory{},
		&EntityMeta{},
		&Keygen{},
		&Passkey{},
		&AuthChallenge{},
		&Job{},
		&ScheduledExecution{},
		&UserMeta{},
		&NodeMeta{},
		&TorrentBlockerReport{},
	}
}

var importedZeroValuesOnce sync.Once
var importedZeroValues map[string]map[string]interface{}

// ImportedZeroValue returns the Go model's zero value for a nullable source
// column whose target field is non-pointer. Pointer fields retain SQL NULL.
func ImportedZeroValue(table, column string) (interface{}, bool) {
	importedZeroValuesOnce.Do(buildImportedZeroValues)
	value, ok := importedZeroValues[table][column]
	return value, ok
}

func buildImportedZeroValues() {
	ns := schema.NamingStrategy{}
	importedZeroValues = make(map[string]map[string]interface{})
	for _, model := range schemaModels() {
		modelType := reflect.TypeOf(model)
		if modelType.Kind() == reflect.Pointer {
			modelType = modelType.Elem()
		}
		tableName := ns.TableName(modelType.Name())
		if named, ok := model.(interface{ TableName() string }); ok {
			tableName = named.TableName()
		}
		if importedZeroValues[tableName] == nil {
			importedZeroValues[tableName] = make(map[string]interface{})
		}
		for i := 0; i < modelType.NumField(); i++ {
			field := modelType.Field(i)
			tag := field.Tag.Get("gorm")
			if tag == "-" || strings.HasPrefix(tag, "-") || strings.Contains(tag, ";-") {
				continue
			}
			fieldColumn := ns.ColumnName("", field.Name)
			for _, part := range strings.Split(tag, ";") {
				if strings.HasPrefix(part, "column:") {
					fieldColumn = strings.TrimPrefix(part, "column:")
					break
				}
			}
			if field.Type.Kind() == reflect.Pointer {
				continue
			}
			switch field.Type.Kind() {
			case reflect.String, reflect.Bool,
				reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
				reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
				reflect.Float32, reflect.Float64, reflect.Struct:
				importedZeroValues[tableName][fieldColumn] = reflect.Zero(field.Type).Interface()
			}
		}
	}

}

func migrateLegacyHostRelations(db *gorm.DB) error {
	columns := make(map[string]struct{})
	columnTypes := make(map[string]string)
	if db.Dialector.Name() == "sqlite" {
		var rows []struct {
			Name string `gorm:"column:name"`
			Type string `gorm:"column:type"`
		}
		if err := db.Raw(`PRAGMA table_info("hosts")`).Scan(&rows).Error; err != nil {
			return err
		}
		for _, row := range rows {
			columns[row.Name] = struct{}{}
			columnTypes[row.Name] = row.Type
		}
	} else if db.Dialector.Name() == "postgres" {
		var rows []struct {
			Name     string `gorm:"column:column_name"`
			DataType string `gorm:"column:data_type"`
			UdtName  string `gorm:"column:udt_name"`
		}
		if err := db.Raw(`SELECT column_name, data_type, udt_name FROM information_schema.columns WHERE table_schema = current_schema() AND table_name = 'hosts'`).Scan(&rows).Error; err != nil {
			return err
		}
		for _, row := range rows {
			columns[row.Name] = struct{}{}
			columnTypes[row.Name] = row.DataType + ":" + row.UdtName
		}
	}
	_, hasNodes := columns["nodes"]
	_, hasSquads := columns["internal_squads"]
	if !hasNodes && !hasSquads {
		return nil
	}

	selects := []string{"uuid"}
	if hasNodes {
		selects = append(selects, legacyHostColumnExpr(db, "nodes", columnTypes["nodes"]))
	}
	if hasSquads {
		selects = append(selects, legacyHostColumnExpr(db, "internal_squads", columnTypes["internal_squads"]))
	}
	rows, err := db.Raw("SELECT " + strings.Join(selects, ", ") + " FROM hosts").Rows()
	if err != nil {
		return fmt.Errorf("read legacy host links: %w", err)
	}
	defer rows.Close()

	type relation struct {
		hostUUID   string
		nodeUUIDs  []string
		squadUUIDs []string
	}
	relations := make([]relation, 0)
	for rows.Next() {
		values := make([]interface{}, len(selects))
		destinations := make([]interface{}, len(selects))
		for i := range values {
			destinations[i] = &values[i]
		}
		if err := rows.Scan(destinations...); err != nil {
			return err
		}
		hostUUID, err := legacyString(values[0])
		if err != nil {
			return err
		}
		index := 1
		var nodeUUIDs, squadUUIDs []string
		if hasNodes {
			raw, err := legacyString(values[index])
			if err != nil {
				return err
			}
			index++
			if err := decodeLegacyUUIDList(raw, &nodeUUIDs); err != nil {
				return fmt.Errorf("decode legacy host node list for host %s: %w", hostUUID, err)
			}
		}
		if hasSquads {
			raw, err := legacyString(values[index])
			if err != nil {
				return err
			}
			if err := decodeLegacyUUIDList(raw, &squadUUIDs); err != nil {
				return fmt.Errorf("decode legacy host squad list for host %s: %w", hostUUID, err)
			}
		}
		relations = append(relations, relation{hostUUID: hostUUID, nodeUUIDs: nodeUUIDs, squadUUIDs: squadUUIDs})
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}

	return db.Transaction(func(tx *gorm.DB) error {
		for _, item := range relations {
			for _, nodeUUID := range item.nodeUUIDs {
				link := HostsToNode{HostUUID: item.hostUUID, NodeUUID: nodeUUID}
				if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&link).Error; err != nil {
					return err
				}
			}
			for _, squadUUID := range item.squadUUIDs {
				link := InternalSquadHostLink{HostUUID: item.hostUUID, SquadUUID: squadUUID}
				if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&link).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func legacyHostColumnExpr(db *gorm.DB, column, dataType string) string {
	if db.Dialector.Name() == "postgres" && (strings.HasPrefix(dataType, "ARRAY:") || strings.Contains(dataType, ":_")) {
		return "to_json(" + column + ")::text AS " + column
	}
	return column
}

func legacyString(value interface{}) (string, error) {
	switch v := value.(type) {
	case nil:
		return "", nil
	case string:
		return v, nil
	case []byte:
		return string(v), nil
	default:
		return "", fmt.Errorf("unexpected legacy value type %T", value)
	}
}

func decodeLegacyUUIDList(raw string, target *[]string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" {
		*target = nil
		return nil
	}
	return json.Unmarshal([]byte(raw), target)
}
