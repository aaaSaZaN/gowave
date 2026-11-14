package nodes

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"gowave/internal/database"
	"gowave/internal/testdb"
)

const rejectedXrayOutput = "invalid Xray configuration: exit status 23: Xray 25.10.15 (Xray, Penetrates Everything.) (go1.25 linux/amd64)\n" +
	"A unified platform for anti-censorship.\n" +
	"Failed to start: main: failed to load config files: [/var/lib/remnanode/config.json.validate] > infra/conf: failed to build inbound config with tag VLESS > json: cannot unmarshal string into Go struct field SplitHTTPConfig.inbounds.streamSettings.xhttpSettings.serverMaxHeaderBytes of type int32\n"

const rejectedXrayReason = "infra/conf: failed to build inbound config with tag VLESS > json: cannot unmarshal string into Go struct field SplitHTTPConfig.inbounds.streamSettings.xhttpSettings.serverMaxHeaderBytes of type int32"

func TestSummarizeXrayError(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{rejectedXrayOutput, rejectedXrayReason},
		{"Failed to start: infra/conf: unknown protocol", "infra/conf: unknown protocol"},
		{"  dial tcp 10.0.0.1:2222: i/o timeout\n", "dial tcp 10.0.0.1:2222: i/o timeout"},
		{strings.Repeat("x", maxStatusMessageLength+5), strings.Repeat("x", maxStatusMessageLength) + "…"},
	} {
		if got := summarizeXrayError(tc.in); got != tc.want {
			t.Errorf("summarizeXrayError(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// fakeXrayNode answers start and health requests like a node agent; other
// requests (user sync after a successful start) get an empty response.
func fakeXrayNode(t *testing.T, startError string, started, xrayRunning bool) *Client {
	t.Helper()
	start := map[string]interface{}{"isStarted": started}
	if startError != "" {
		start["error"] = startError
	}
	startBody, err := json.Marshal(map[string]interface{}{"response": start})
	if err != nil {
		t.Fatal(err)
	}
	healthBody, err := json.Marshal(map[string]interface{}{"response": map[string]interface{}{"isAlive": true, "xrayInternalStatusCached": xrayRunning}})
	if err != nil {
		t.Fatal(err)
	}
	return &Client{httpClient: &http.Client{Transport: nodeRoundTrip(func(r *http.Request) (*http.Response, error) {
		body := `{"response":{}}`
		switch r.URL.Path {
		case "/node/xray/start":
			body = string(startBody)
		case "/node/xray/healthcheck":
			body = string(healthBody)
		}
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}}
}

func TestStartNodeKeepsServingNodeWhenNewConfigIsRejected(t *testing.T) {
	for _, tc := range []struct {
		name        string
		startError  string
		xrayRunning bool
		wantRetry   bool
		wantStatus  *string // stored last_status_message
		wantConfig  *string // stored xray_config_error
		wantOnline  bool
	}{
		{name: "invalid config, previous Xray running", startError: rejectedXrayOutput, xrayRunning: true, wantConfig: strPtr(rejectedXrayReason), wantOnline: true},
		{name: "transient error, previous Xray running", startError: "Request already in progress", xrayRunning: true, wantRetry: true, wantConfig: strPtr("Request already in progress"), wantOnline: true},
		{name: "invalid config, Xray stopped", startError: rejectedXrayOutput, wantRetry: true, wantStatus: strPtr(rejectedXrayReason)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db := testdb.Open(t)
			node := database.Node{UUID: "node", Name: "node", Address: "example.com", IsConnected: true}
			if err := db.Create(&node).Error; err != nil {
				t.Fatal(err)
			}
			service := NewService(db, fakeXrayNode(t, tc.startError, false, tc.xrayRunning))

			err := service.ApplyNodeJob(context.Background(), &database.Job{Kind: "node-start", Target: node.UUID})
			if (err != nil) != tc.wantRetry {
				t.Fatalf("ApplyNodeJob error = %v, want retry %v", err, tc.wantRetry)
			}
			if err := db.Where("uuid = ?", node.UUID).First(&node).Error; err != nil {
				t.Fatal(err)
			}
			if node.IsConnected != tc.wantOnline || node.IsConnecting {
				t.Fatalf("connected=%v connecting=%v, want connected=%v", node.IsConnected, node.IsConnecting, tc.wantOnline)
			}
			if !equalPtr(node.LastStatusMessage, tc.wantStatus) || !equalPtr(node.XrayConfigError, tc.wantConfig) {
				t.Fatalf("status=%v configError=%v, want %v and %v", deref(node.LastStatusMessage), deref(node.XrayConfigError), deref(tc.wantStatus), deref(tc.wantConfig))
			}
			wantMessage := tc.wantStatus
			if wantMessage == nil {
				wantMessage = tc.wantConfig
			}
			if got, _ := formatNodeResponse(service, &node)["lastStatusMessage"].(*string); !equalPtr(got, wantMessage) {
				t.Fatalf("lastStatusMessage = %v, want %v", deref(got), deref(wantMessage))
			}
		})
	}
}

func TestConfigErrorSurvivesHealthChecksUntilStartSucceeds(t *testing.T) {
	db := testdb.Open(t)
	node := database.Node{UUID: "node", Name: "node", Address: "example.com", IsConnected: true, XrayConfigError: strPtr(rejectedXrayReason)}
	if err := db.Create(&node).Error; err != nil {
		t.Fatal(err)
	}

	NewService(db, fakeXrayNode(t, "", false, true)).CheckNodeHealth(&node)
	if err := db.Where("uuid = ?", node.UUID).First(&node).Error; err != nil {
		t.Fatal(err)
	}
	if !node.IsConnected || !equalPtr(node.XrayConfigError, strPtr(rejectedXrayReason)) {
		t.Fatalf("health check changed connected=%v configError=%v", node.IsConnected, deref(node.XrayConfigError))
	}

	if ok, err := NewService(db, fakeXrayNode(t, "", true, true)).StartNode(&node, true); !ok || err != nil {
		t.Fatalf("StartNode = %v, %v", ok, err)
	}
	if err := db.Where("uuid = ?", node.UUID).First(&node).Error; err != nil {
		t.Fatal(err)
	}
	if node.XrayConfigError != nil {
		t.Fatalf("successful start kept configError %q", *node.XrayConfigError)
	}
}

func TestDisablingNodeClearsConfigError(t *testing.T) {
	db := testdb.Open(t)
	node := database.Node{UUID: "node", Name: "node", Address: "example.com", IsConnected: true, XrayConfigError: strPtr(rejectedXrayReason)}
	if err := db.Create(&node).Error; err != nil {
		t.Fatal(err)
	}
	updated, err := NewService(db, nil).SetDisabled(node.UUID, true)
	if err != nil {
		t.Fatal(err)
	}
	if updated.XrayConfigError != nil {
		t.Fatalf("disabled node kept configError %q", *updated.XrayConfigError)
	}
}

func strPtr(s string) *string { return &s }

func deref(s *string) string {
	if s == nil {
		return "<nil>"
	}
	return *s
}

func equalPtr(a, b *string) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
}
