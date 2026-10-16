package nodes

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"testing"

	"gowave/internal/database"
	"gowave/internal/testdb"
)

// TestGetNodesBatchesInboundsLikeSingleNode checks that the batched list keeps
// the per-node rules: linked inbounds win, otherwise the whole active profile.
func TestGetNodesBatchesInboundsLikeSingleNode(t *testing.T) {
	db := testdb.Open(t)
	strPtr := func(s string) *string { return &s }

	fixtures := []interface{}{
		&database.ConfigProfile{UUID: "cp-a", Name: "Profile A", Config: "{}"},
		&database.ConfigProfile{UUID: "cp-b", Name: "Profile B", Config: "{}"},
		&database.ConfigProfileInbound{UUID: "in-a1", ProfileUUID: "cp-a", Tag: "A1", Type: "vless", RawInbound: `{"tag":"A1"}`},
		&database.ConfigProfileInbound{UUID: "in-a2", ProfileUUID: "cp-a", Tag: "A2", Type: "vless", RawInbound: `{"tag":"A2"}`},
		&database.ConfigProfileInbound{UUID: "in-b1", ProfileUUID: "cp-b", Tag: "B1", Type: "trojan", RawInbound: `{"tag":"B1"}`},
		// Linked to one inbound of its profile.
		&database.Node{UUID: "node-linked", Name: "linked", Address: "10.0.0.1", ActiveConfigProfileUUID: strPtr("cp-a")},
		// No links: falls back to every inbound of the profile.
		&database.Node{UUID: "node-fallback", Name: "fallback", Address: "10.0.0.2", ActiveConfigProfileUUID: strPtr("cp-a")},
		&database.Node{UUID: "node-other", Name: "other", Address: "10.0.0.3", ActiveConfigProfileUUID: strPtr("cp-b")},
		&database.Node{UUID: "node-none", Name: "none", Address: "10.0.0.4"},
		&database.ConfigProfileInboundsToNodes{ConfigProfileInboundUUID: "in-a2", NodeUUID: "node-linked"},
	}
	for _, fixture := range fixtures {
		if err := db.Create(fixture).Error; err != nil {
			t.Fatalf("create %T: %v", fixture, err)
		}
	}

	service := NewService(db, nil)
	handler := NewHandler(service)
	recorder := httptest.NewRecorder()
	handler.GetNodes(recorder, httptest.NewRequest(http.MethodGet, "/api/nodes", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("GET /api/nodes = %d: %s", recorder.Code, recorder.Body.String())
	}
	var list struct {
		Response []map[string]interface{} `json:"response"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}

	want := map[string][]string{
		"node-linked":   {"A2"},
		"node-fallback": {"A1", "A2"},
		"node-other":    {"B1"},
		"node-none":     {},
	}
	if len(list.Response) != len(want) {
		t.Fatalf("got %d nodes, want %d", len(list.Response), len(want))
	}
	for _, item := range list.Response {
		uuid, _ := item["uuid"].(string)
		if got := activeInboundTags(t, item); !reflect.DeepEqual(got, want[uuid]) {
			t.Errorf("%s: active inbounds %v, want %v", uuid, got, want[uuid])
		}

		node, err := service.GetByUUID(uuid)
		if err != nil {
			t.Fatal(err)
		}
		single := roundTripJSON(t, formatNodeResponse(service, node))
		if !reflect.DeepEqual(single["configProfile"], item["configProfile"]) {
			t.Errorf("%s: list configProfile %v differs from single node %v", uuid, item["configProfile"], single["configProfile"])
		}
	}
}

func activeInboundTags(t *testing.T, item map[string]interface{}) []string {
	t.Helper()
	profile, _ := item["configProfile"].(map[string]interface{})
	inbounds, _ := profile["activeInbounds"].([]interface{})
	tags := []string{}
	for _, raw := range inbounds {
		inbound, _ := raw.(map[string]interface{})
		tag, _ := inbound["tag"].(string)
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	return tags
}

func roundTripJSON(t *testing.T, value interface{}) map[string]interface{} {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]interface{}
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	return out
}
