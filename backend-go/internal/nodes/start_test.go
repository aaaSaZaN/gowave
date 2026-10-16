package nodes

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"
	"gowave/internal/database"
	"gowave/internal/testdb"
)

type nodeRoundTrip func(*http.Request) (*http.Response, error)

func (f nodeRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestStartNodeClearsConnectingOnPreparationFailure(t *testing.T) {
	for _, scenario := range []string{"config expansion", "user population", "integration lookup", "integration decoding", "plugin generation", "plugin rejection"} {
		t.Run(scenario, func(t *testing.T) {
			db := testdb.Open(t)
			node := database.Node{UUID: "node", Name: "node", Address: "example.com"}
			profile := database.ConfigProfile{UUID: "profile", Name: "profile", Config: `{}`}
			plugin := database.NodePlugin{UUID: "plugin", Name: "plugin", PluginConfig: `{}`}
			switch scenario {
			case "config expansion":
				profile.Config = `invalid JSON`
				node.ActiveConfigProfileUUID = &profile.UUID
			case "user population":
				profile.Config = `{"inbounds":[{"tag":"test","protocol":"vless"}]}`
				node.ActiveConfigProfileUUID = &profile.UUID
				if err := db.Callback().Query().Before("gorm:query").Register("test:fail-user-query", func(tx *gorm.DB) {
					if tx.Statement.Table == "users" {
						tx.AddError(errors.New("user population failed"))
					}
				}); err != nil {
					t.Fatal(err)
				}
			case "integration lookup", "integration decoding":
				node.IntegrationUUIDs = database.StringArray{"11111111-1111-4111-8111-111111111111"}
				if scenario == "integration decoding" {
					if err := db.Create(&database.Integration{UUID: "11111111-1111-4111-8111-111111111111", Name: "integration", Config: `invalid JSON`}).Error; err != nil {
						t.Fatal(err)
					}
				}
			case "plugin generation", "plugin rejection":
				node.ActivePluginUUID = &plugin.UUID
				if scenario == "plugin generation" {
					plugin.PluginConfig = `invalid JSON`
				}
			}
			for _, record := range []interface{}{&profile, &plugin, &node} {
				if err := db.Create(record).Error; err != nil {
					t.Fatal(err)
				}
			}
			client := &Client{httpClient: &http.Client{Transport: nodeRoundTrip(func(r *http.Request) (*http.Response, error) {
				if r.URL.Path != "/node/plugin/sync" || scenario != "plugin rejection" {
					t.Fatalf("unexpected request %s", r.URL.Path)
				}
				return &http.Response{StatusCode: http.StatusBadRequest, Body: io.NopCloser(strings.NewReader(""))}, nil
			})}}
			ok, err := NewService(db, client).StartNode(&node, false)
			if ok || err == nil {
				t.Fatalf("preparation succeeded: %v, %v", ok, err)
			}
			if err := db.Where("uuid = ?", node.UUID).First(&node).Error; err != nil {
				t.Fatal(err)
			}
			if node.IsConnecting {
				t.Fatal("node remained connecting after failed preparation")
			}
		})
	}
}

func TestRepeatedHealthChecksDeduplicateStartJobs(t *testing.T) {
	db := testdb.Open(t)
	node := database.Node{UUID: "node", Name: "node", Address: "example.com"}
	if err := db.Create(&node).Error; err != nil {
		t.Fatal(err)
	}
	client := &Client{httpClient: &http.Client{Transport: nodeRoundTrip(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"response":{"isAlive":false}}`))}, nil
	})}}
	service := NewService(db, client)
	minute := time.Now().Unix() / 60
	for i := 0; i < 3; i++ {
		service.CheckNodeHealth(&node)
	}
	if time.Now().Unix()/60 != minute {
		t.Skip("health checks crossed a minute boundary")
	}
	var count int64
	if err := db.Model(&database.Job{}).Where("kind = ?", "node-start").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("enqueued %d jobs for the same minute", count)
	}
}
