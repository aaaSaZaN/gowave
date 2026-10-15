package prometheus

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"remnawave-go/internal/database"
	"remnawave-go/internal/metrics"
	"remnawave-go/internal/nodes"
	"remnawave-go/internal/testdb"
)

type hotStats map[string]*nodes.NodeHotMetrics

func (h hotStats) GetNodeMetrics(uuid string) *nodes.NodeHotMetrics { return h[uuid] }

func TestMissingInterfaceRemovesOnlyNetworkMetrics(t *testing.T) {
	db := testdb.Open(t)
	node := database.Node{UUID: "no-interface", Name: "node", Address: "example.com"}
	if err := db.Create(&node).Error; err != nil {
		t.Fatal(err)
	}
	var system nodes.NodeSystemStatsResponse
	if err := json.Unmarshal([]byte(`{"system":{"info":{"memoryTotal":100,"cpus":4},"stats":{"memoryFree":50,"uptime":123,"loadAvg":[1,2,3],"interface":{"rxTotal":10,"txTotal":20}}}}`), &system); err != nil {
		t.Fatal(err)
	}
	exporter := NewExporter(db, hotStats{node.UUID: {System: &system}})
	exporter.reportNodesStats()
	system.System.Stats.Interface = nil
	exporter.reportNodesStats()
	text := metrics.Default.Text()
	for _, name := range []string{
		"node_memory_total_bytes", "node_memory_free_bytes", "node_uptime_seconds", "node_cpu_count",
		"node_cpu_load_avg_1m", "node_cpu_load_avg_5m", "node_cpu_load_avg_15m",
	} {
		if !strings.Contains(text, "remnawave_"+name+`{node_uuid="no-interface",`) {
			t.Errorf("missing system gauge %s", name)
		}
	}
	for _, name := range []string{
		"node_network_rx_bytes_per_sec", "node_network_tx_bytes_per_sec", "node_network_rx_bytes_total", "node_network_tx_bytes_total",
	} {
		if strings.Contains(text, "remnawave_"+name+`{node_uuid="no-interface",`) {
			t.Errorf("stale network gauge %s", name)
		}
	}
}

func TestExporterAndServer(t *testing.T) {
	db := testdb.Open(t)
	now := time.Now().UTC()
	online := now.Add(-10 * time.Second)
	for i, status := range []string{"ACTIVE", "ACTIVE", "LIMITED"} {
		user := database.User{ID: uint64(100 + i), ShortUUID: "short" + status + string(rune('a'+i)), Username: "user" + string(rune('a'+i)), Status: status, ExpireAt: now.Add(time.Hour), CreatedAt: now, UpdatedAt: now}
		if err := db.Create(&user).Error; err != nil {
			t.Fatal(err)
		}
		traffic := database.UserTraffic{ID: user.ID}
		if i == 0 {
			traffic.OnlineAt = &online
		}
		if err := db.Create(&traffic).Error; err != nil {
			t.Fatal(err)
		}
	}
	node := database.Node{UUID: "33333333-3333-4333-8333-333333333333", Name: "de-1", Address: "1.2.3.4", CountryCode: "DE", IsConnected: true, Tags: database.StringArray{"EU", "FAST"}}
	if err := db.Create(&node).Error; err != nil {
		t.Fatal(err)
	}
	metrics.RecordNodeTraffic("deleted-node", []metrics.NodeTraffic{{Tag: "in", Uplink: 1}}, nil)
	metrics.RecordNodeTraffic(node.UUID, []metrics.NodeTraffic{{Tag: "vless", Uplink: 10, Downlink: 20}}, []metrics.NodeTraffic{{Tag: "direct", Uplink: 5}})

	exporter := NewExporter(db, hotStats{node.UUID: {OnlineUsers: 4}})
	exporter.Export()
	exporter.Sync()
	text := metrics.Default.Text()
	for _, want := range []string{
		`remnawave_users_status{status="ACTIVE",app="remnawave"} 2`,
		`remnawave_users_status{status="LIMITED",app="remnawave"} 1`,
		`remnawave_users_online_stats{metricType="onlineNow",app="remnawave"} 1`,
		`remnawave_users_online_stats{metricType="neverOnline",app="remnawave"} 2`,
		`remnawave_users_total{type="all",app="remnawave"} 3`,
		`remnawave_node_status{node_uuid="33333333-3333-4333-8333-333333333333",app="remnawave"} 1`,
		`remnawave_node_online_users{node_uuid="33333333-3333-4333-8333-333333333333",app="remnawave"} 4`,
		`remnawave_node_basic_info{node_uuid="33333333-3333-4333-8333-333333333333",node_name="de-1",node_country_emoji="` + CountryEmoji("DE") + `",provider_name="unknown",tags="EU,FAST",app="remnawave"} 1`,
		`remnawave_node_inbound_download_bytes{node_uuid="33333333-3333-4333-8333-333333333333",tag="vless",app="remnawave"} 20`,
		`remnawave_process_active_handles{instance_id="0",instance_name="api",app="remnawave"}`,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %s", want)
		}
	}
	if strings.Contains(text, "deleted-node") {
		t.Error("sync kept a deleted node")
	}
	if CountryEmoji("XX") != CountryEmoji("1") || CountryEmoji("de") != CountryEmoji("DE") {
		t.Error("country emoji")
	}

	server := httptest.NewServer(Handler(db, "prom", "secret"))
	defer server.Close()
	get := func(path, user, pass string) (*http.Response, error) {
		request, _ := http.NewRequest("GET", server.URL+path, nil)
		if user != "" {
			request.SetBasicAuth(user, pass)
		}
		return http.DefaultClient.Do(request)
	}
	response, err := get("/metrics", "prom", "wrong")
	if err != nil || response.StatusCode != 401 || response.Header.Get("WWW-Authenticate") != `Basic realm="Prometheus Metrics"` {
		t.Fatalf("unauthorized: %v %v", response, err)
	}
	response, err = get("/metrics", "prom", "secret")
	if err != nil || response.StatusCode != 200 || response.Header.Get("Content-Type") != metrics.ContentType {
		t.Fatalf("metrics: %v %v", response, err)
	}
	response, err = get("/health", "", "")
	if err != nil || response.StatusCode != 200 {
		t.Fatalf("health: %v %v", response, err)
	}
	if _, err := get("/metrics/", "prom", "secret"); err == nil {
		t.Fatal("other paths must drop the connection")
	}
}
