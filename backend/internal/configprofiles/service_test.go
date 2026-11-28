package configprofiles

import (
	"encoding/json"
	"reflect"
	"testing"

	"gowave/internal/database"
	"gowave/internal/testdb"
)

func TestUpdateProfileDetachesHostsFromRenamedInbound(t *testing.T) {
	service := NewService(testdb.Open(t))
	profile, err := service.CreateProfile("test profile", []byte(`{
		"inbounds": [
			{"tag":"old","protocol":"vless","port":443},
			{"tag":"kept","protocol":"vless","port":444}
		]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	inbounds, err := service.GetInboundsByProfileUUID(profile.UUID)
	if err != nil {
		t.Fatal(err)
	}
	byTag := make(map[string]string)
	hosts := make(map[string]string)
	for _, inbound := range inbounds {
		byTag[inbound.Tag] = inbound.UUID
		host, err := service.CreateHost(CreateHostDTO{
			Remark: inbound.Tag, Address: inbound.Tag + ".example.com", Port: 443,
			ConfigProfileUUID: profile.UUID, ConfigProfileInboundUUID: inbound.UUID,
		})
		if err != nil {
			t.Fatal(err)
		}
		hosts[inbound.Tag] = host.UUID
	}
	if _, err := service.UpdateProfile(profile.UUID, nil, []byte(`{
		"inbounds": [
			{"tag":"renamed","protocol":"vless","port":443},
			{"tag":"kept","protocol":"vless","port":444}
		]
	}`)); err != nil {
		t.Fatalf("rename inbound: %v", err)
	}
	inbounds, err = service.GetInboundsByProfileUUID(profile.UUID)
	if err != nil {
		t.Fatal(err)
	}
	updated := make(map[string]string)
	for _, inbound := range inbounds {
		updated[inbound.Tag] = inbound.UUID
	}
	if len(updated) != 2 || updated["kept"] != byTag["kept"] || updated["renamed"] == "" {
		t.Fatalf("unexpected inbounds after rename: %v", updated)
	}
	for _, tag := range []string{"old", "kept"} {
		host, err := service.GetHostByUUID(hosts[tag])
		if err != nil {
			t.Fatal(err)
		}
		if host.ConfigProfileUUID == nil || *host.ConfigProfileUUID != profile.UUID || host.Remark != tag {
			t.Fatalf("rename changed the host or its profile: %+v", host)
		}
		if tag == "old" && host.ConfigProfileInboundUUID != nil {
			t.Fatalf("host kept an inbound after rename: %q", *host.ConfigProfileInboundUUID)
		}
		if tag == "kept" && (host.ConfigProfileInboundUUID == nil || *host.ConfigProfileInboundUUID != byTag[tag]) {
			t.Fatal("rename cleared an unrelated host's inbound")
		}
	}
}

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
