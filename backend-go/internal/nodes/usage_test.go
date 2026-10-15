package nodes

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"remnawave-go/config"
	"remnawave-go/internal/database"
	"remnawave-go/internal/notify"
	"remnawave-go/internal/testdb"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func usageDB(t *testing.T) *gorm.DB { return testdb.Open(t) }

func TestMultiplyConsumptionMatchesUpstream(t *testing.T) {
	for _, c := range []struct{ nano, bytes, want int64 }{
		{0, 1000, 0}, {1_000_000_000, 1000, 1000}, {1_500_000_000, 1001, 1501}, {500_000_000, 3, 1},
	} {
		if got := multiplyConsumption(c.nano, c.bytes); got != c.want {
			t.Errorf("multiplyConsumption(%d, %d) = %d, want %d", c.nano, c.bytes, got, c.want)
		}
	}
}

func TestRecordUserUsageBillsHistoryAndFirstConnection(t *testing.T) {
	db := usageDB(t)
	notifier, _ := notify.New(&config.Config{Notifications: config.NotificationsConfig{WebhookEnabled: true, WebhookURLs: []string{"https://h.example.com"}, WebhookSecretHeader: strings.Repeat("s", 32)}})
	notify.SetDefault(notifier)
	t.Cleanup(func() { notify.SetDefault(nil) })

	node := database.Node{UUID: uuid.NewString(), Name: "n1", Address: "192.0.2.1", ConsumptionMultiplier: 2_000_000_000}
	if err := db.Create(&node).Error; err != nil {
		t.Fatal(err)
	}
	var ids []uint64
	for i := 0; i < 3; i++ {
		user := database.User{ShortUUID: uuid.NewString()[:16], Username: "u" + string(rune('a'+i)), Status: "ACTIVE", ExpireAt: time.Now().Add(time.Hour), VlessUUID: uuid.NewString()}
		db.Create(&user)
		db.Create(&database.UserTraffic{ID: user.ID})
		ids = append(ids, user.ID)
	}
	s := &Service{db: db, usage: UsageOptions{IgnoreBelowBytes: 10}}
	now := time.Now().UTC()
	stats := []NodeUserTraffic{
		{Username: itoa(ids[0]), Uplink: 100, Downlink: 50},
		{Username: itoa(ids[1]), Uplink: 3, Downlink: 2}, // below the ignore threshold
		{Username: "not-a-user", Uplink: 1000},
	}
	if online := s.recordUserUsage(&node, stats, now); online != 1 {
		t.Fatalf("online = %d, want 1", online)
	}
	s.recordUserUsage(&node, []NodeUserTraffic{{Username: itoa(ids[0]), Uplink: 10}}, now)

	var traffic database.UserTraffic
	db.First(&traffic, ids[0])
	if traffic.UsedTrafficBytes != 320 || traffic.LifetimeUsedTrafficBytes != 320 || traffic.FirstConnectedAt == nil {
		t.Fatalf("traffic = %+v, want 320 billed bytes with first connection", traffic)
	}
	var ignored database.UserTraffic
	db.First(&ignored, ids[1])
	if ignored.UsedTrafficBytes != 0 || ignored.FirstConnectedAt != nil {
		t.Fatal("traffic below USER_USAGE_IGNORE_BELOW_BYTES was recorded")
	}
	var history database.NodesUserUsageHistory
	db.Where("user_id = ?", ids[0]).First(&history)
	if history.TotalBytes != 160 {
		t.Fatalf("history = %d raw bytes, want 160", history.TotalBytes)
	}
	var events int64
	db.Model(&database.Job{}).Where("kind = 'webhook' AND entity_id = 'user.first_connected'").Count(&events)
	if events != 1 {
		t.Fatalf("first_connected events = %d, want 1", events)
	}
}

func itoa(id uint64) string { return strconv.FormatUint(id, 10) }

const sampleTorrentReport = `{"actionReport":{"blocked":true,"ip":"198.51.100.20","blockDuration":3600,"willUnblockAt":"2026-03-04T13:00:00Z","userId":"%d","processedAt":"2026-03-04T12:00:00Z"},"xrayReport":{"email":"%d","level":0,"protocol":"bittorrent","network":"tcp","source":"198.51.100.20:5000","destination":"203.0.113.9:6881","routeTarget":null,"originalTarget":null,"inboundTag":"VLESS","inboundName":null,"inboundLocal":null,"outboundTag":"BLOCK","ts":1772625600}}`

func TestStoreTorrentReportsPersistsAndNotifies(t *testing.T) {
	db := usageDB(t)
	notifier, _ := notify.New(&config.Config{Notifications: config.NotificationsConfig{WebhookEnabled: true, WebhookURLs: []string{"https://h.example.com"}, WebhookSecretHeader: strings.Repeat("s", 32)}})
	notify.SetDefault(notifier)
	t.Cleanup(func() { notify.SetDefault(nil) })

	node := database.Node{UUID: uuid.NewString(), Name: "n1", Address: "192.0.2.1"}
	db.Create(&node)
	user := database.User{ShortUUID: uuid.NewString()[:16], Username: "seeder", Status: "ACTIVE", ExpireAt: time.Now().Add(time.Hour), VlessUUID: uuid.NewString()}
	db.Create(&user)
	reports := []json.RawMessage{
		json.RawMessage(fmt.Sprintf(sampleTorrentReport, user.ID, user.ID)),
		json.RawMessage(fmt.Sprintf(sampleTorrentReport, 999999, 999999)), // unknown user: dropped
	}
	s := &Service{db: db, nodeMetrics: map[string]*NodeHotMetrics{}}
	if err := s.storeTorrentReports(&node, reports); err != nil {
		t.Fatal(err)
	}
	var stored []database.TorrentBlockerReport
	db.Find(&stored)
	if len(stored) != 1 || stored[0].UserID != user.ID || stored[0].NodeID != node.ID {
		t.Fatalf("stored = %+v", stored)
	}
	var events int64
	db.Model(&database.Job{}).Where("kind = 'webhook' AND entity_id = 'torrent_blocker.report'").Count(&events)
	if events != 1 {
		t.Fatalf("events = %d, want 1", events)
	}
}
