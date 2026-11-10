package configprofiles

import (
	"encoding/json"
	"reflect"
	"testing"

	"gowave/internal/database"
	"gowave/internal/testdb"
)

func TestHostResponseRuleRestrictions(t *testing.T) {
	db := testdb.Open(t)
	service := NewService(db)
	host, err := service.CreateHostFromInput(CreateHostInput{
		Remark: "rules", Address: "example.com", Port: 443,
		IncludeInSrrRules: []string{"First rule", "Second rule"},
	})
	if err != nil {
		t.Fatal(err)
	}
	assertRules := func(id string, want []string) {
		t.Helper()
		stored, err := service.GetHostByUUID(id)
		if err != nil {
			t.Fatal(err)
		}
		response, err := formatHostResponse(db, stored)
		if err != nil || !reflect.DeepEqual(response["includeInSrrRules"], want) {
			t.Fatalf("rules = %v, want %v (error %v)", response["includeInSrrRules"], want, err)
		}
	}
	assertRules(host.UUID, []string{"First rule", "Second rule"})
	clone, err := service.CloneHost(host.UUID)
	if err != nil {
		t.Fatal(err)
	}
	assertRules(clone.UUID, []string{"First rule", "Second rule"})
	for _, tc := range []struct {
		patch map[string]json.RawMessage
		want  []string
	}{
		{map[string]json.RawMessage{"includeInSrrRules": json.RawMessage(`["Other rule"]`)}, []string{"Other rule"}},
		{map[string]json.RawMessage{"remark": json.RawMessage(`"renamed"`)}, []string{"Other rule"}},
		{map[string]json.RawMessage{"includeInSrrRules": json.RawMessage(`[]`)}, []string{}},
	} {
		if _, err := service.PatchHost(host.UUID, tc.patch); err != nil {
			t.Fatal(err)
		}
		assertRules(host.UUID, tc.want)
	}
	for _, invalid := range []string{`[123]`, `null`, `"Selected"`} {
		if _, err := service.PatchHost(host.UUID, map[string]json.RawMessage{
			"includeInSrrRules": json.RawMessage(invalid),
		}); err == nil {
			t.Fatalf("invalid rules accepted: %s", invalid)
		}
	}
	assertRules(host.UUID, []string{})
}

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
