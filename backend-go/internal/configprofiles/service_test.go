package configprofiles

import (
	"encoding/json"
	"testing"

	"remnawave-go/internal/database"
	"remnawave-go/internal/testdb"
)

func TestPatchHostCorrectsStoredModeAndRollsBackInvalidRelations(t *testing.T) {
	db := testdb.Open(t)
	host := database.Host{UUID: "host", Remark: "original", Address: "example.com", Port: 443, InternalSquadsMode: "INVALID"}
	if err := db.Create(&host).Error; err != nil {
		t.Fatal(err)
	}
	service := NewService(db)
	updated, err := service.PatchHost(host.UUID, map[string]json.RawMessage{
		"internalSquads": json.RawMessage(`{"mode":"ALLOW_ONLY","squads":[]}`),
	})
	if err != nil || updated.InternalSquadsMode != "ALLOW_ONLY" {
		t.Fatalf("mode correction failed: %+v, %v", updated, err)
	}
	if _, err := service.PatchHost(host.UUID, map[string]json.RawMessage{
		"remark": json.RawMessage(`"changed"`), "nodes": json.RawMessage(`123`),
	}); err == nil {
		t.Fatal("invalid nodes accepted")
	}
	saved, err := service.GetHostByUUID(host.UUID)
	if err != nil || saved.Remark != "original" || saved.InternalSquadsMode != "ALLOW_ONLY" {
		t.Fatalf("unexpected persisted host: %+v, %v", saved, err)
	}
}
