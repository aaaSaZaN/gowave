package nodes

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"gowave/internal/database"
	"gowave/internal/testdb"

	"github.com/go-chi/chi/v5"
)

func TestRestartAllNodesMode(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		force      bool
		status     int
	}{
		{"graceful", `{"forceRestart":false}`, false, http.StatusOK},
		{"forced", `{"forceRestart":true}`, true, http.StatusOK},
		{"legacy client", "", true, http.StatusOK},
		{"invalid mode", `{"forceRestart":"false"}`, false, http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := testdb.Open(t)
			node := database.Node{UUID: "node", Name: "node", Address: "example.com"}
			if err := db.Create(&node).Error; err != nil {
				t.Fatal(err)
			}
			mode := make(chan *bool, 1)
			client := &Client{httpClient: &http.Client{Transport: nodeRoundTrip(func(r *http.Request) (*http.Response, error) {
				reply := httptest.NewRecorder()
				if r.URL.Path == "/node/xray/start" {
					var payload struct{ Internals struct{ ForceRestart *bool } }
					if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
						t.Error(err)
					}
					mode <- payload.Internals.ForceRestart
					// A failed start avoids background user sync after the test ends.
					_, _ = reply.WriteString(`{"response":{"isStarted":false}}`)
				} else {
					_, _ = reply.WriteString(`{"response":{"isAlive":false}}`)
				}
				return reply.Result(), nil
			})}}
			handler := NewHandler(NewService(db, client))
			recorder := httptest.NewRecorder()
			handler.RestartAllNodes(recorder, httptest.NewRequest(http.MethodPost, "/api/nodes/actions/restart-all", strings.NewReader(tc.body)))
			if recorder.Code != tc.status {
				t.Fatalf("restart all: got %d, want %d: %s", recorder.Code, tc.status, recorder.Body)
			}
			if tc.status != http.StatusOK {
				return
			}
			select {
			case got := <-mode:
				if got == nil || *got != tc.force {
					t.Errorf("node did not receive forceRestart=%v", tc.force)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("node did not receive a restart request")
			}
			deadline := time.Now().Add(5 * time.Second)
			for {
				var stored database.Node
				if err := db.First(&stored, "uuid = ?", node.UUID).Error; err != nil {
					t.Fatal(err)
				}
				if stored.LastStatusChange != nil {
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("restart did not finish")
				}
				time.Sleep(10 * time.Millisecond)
			}
		})
	}
}

func TestLocalCoreStatusAndUpdateConflict(t *testing.T) {
	db := testdb.Open(t)
	node := database.Node{UUID: "local-core", Name: "Local Core", Address: "example.com", IsConnected: true}
	if err := db.Create(&node).Error; err != nil {
		t.Fatal(err)
	}
	coreSource := "local"
	client := &Client{httpClient: &http.Client{Transport: nodeRoundTrip(func(r *http.Request) (*http.Response, error) {
		reply := httptest.NewRecorder()
		switch r.URL.Path {
		case "/node/xray/healthcheck":
			if r.Header.Get("X-Better-One") != "true" {
				t.Fatal("health request missing X-Better-One")
			}
			_, _ = reply.WriteString(`{"response":{"isAlive":true,"xrayInternalStatusCached":true,"coreSource":"` + coreSource + `"}}`)
		case "/node/updates/apply":
			var update struct{ Type string }
			if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
				t.Fatal(err)
			}
			switch {
			case update.Type == "node":
				reply.WriteHeader(http.StatusServiceUnavailable)
				_, _ = reply.WriteString(`{"error":"releases unavailable"}`)
			case coreSource == "local":
				reply.WriteHeader(http.StatusConflict)
				_, _ = reply.WriteString(`{"error":"Xray core is managed locally"}`)
			default:
				reply.WriteHeader(http.StatusConflict)
				_, _ = reply.WriteString(`{"error":"another update is in progress"}`)
			}
		default:
			t.Fatalf("unexpected node request %s", r.URL.Path)
		}
		return reply.Result(), nil
	})}}
	service := NewService(db, client)
	handler := NewHandler(service)
	router := chi.NewRouter()
	router.Get("/api/nodes/{uuid}", handler.GetNode)
	router.Post("/api/nodes/{uuid}/updates/apply", handler.ApplyNodeUpdate)
	const path = "/api/nodes/local-core"

	service.CheckNodeHealth(&node)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
	var details struct {
		Response struct{ Versions NodeVersionsResponse }
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &details); err != nil {
		t.Fatal(err)
	}
	if recorder.Code != http.StatusOK || details.Response.Versions.CoreSource != "local" {
		t.Fatalf("GET node: %d %s", recorder.Code, recorder.Body)
	}

	for _, tc := range []struct {
		name, source, body, message string
		status                      int
	}{
		{"custom URL", "local", `{"type":"core","downloadUrl":"https://example.com/core.zip"}`, "GW_NODE_LOCAL_CORE", http.StatusConflict},
		{"Xray release", "local", `{"type":"core","version":"v1"}`, "GW_NODE_LOCAL_CORE", http.StatusConflict},
		{"node binary", "local", `{"type":"node","version":"v1"}`, "releases unavailable", http.StatusBadGateway},
		{"panel can manage the core after the override is removed", "panel", `{"type":"core","version":"v1"}`, "another update is in progress", http.StatusBadGateway},
	} {
		t.Run(tc.name, func(t *testing.T) {
			coreSource = tc.source
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, path+"/updates/apply", strings.NewReader(tc.body)))
			if recorder.Code != tc.status || !strings.Contains(recorder.Body.String(), tc.message) {
				t.Fatalf("POST update: %d %s", recorder.Code, recorder.Body)
			}
		})
	}
}

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
