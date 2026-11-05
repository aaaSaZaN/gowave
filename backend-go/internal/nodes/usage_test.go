package nodes

import (
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"gowave/config"
	"gowave/internal/database"
	"gowave/internal/notify"
	"gowave/internal/testdb"

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

func usageUsers(t *testing.T, db *gorm.DB, n int) []uint64 {
	t.Helper()
	ids := make([]uint64, 0, n)
	for i := 0; i < n; i++ {
		user := database.User{ShortUUID: uuid.NewString()[:16], Username: "u" + uuid.NewString()[:12], Status: "ACTIVE", ExpireAt: time.Now().Add(time.Hour), VlessUUID: uuid.NewString()}
		if err := db.Create(&user).Error; err != nil {
			t.Fatal(err)
		}
		db.Create(&database.UserTraffic{ID: user.ID, FirstConnectedAt: &user.CreatedAt})
		ids = append(ids, user.ID)
	}
	return ids
}

func usedBytes(db *gorm.DB, ids []uint64) int64 {
	var total int64
	db.Model(&database.UserTraffic{}).Where("id IN ?", ids).Select("COALESCE(SUM(used_traffic_bytes), 0)").Scan(&total)
	return total
}

func TestRecordUserUsageRetriesFailedBatchOnNextPoll(t *testing.T) {
	db := usageDB(t)
	node := database.Node{UUID: uuid.NewString(), Name: "n1", Address: "192.0.2.2", ConsumptionMultiplier: 1_000_000_000}
	db.Create(&node)
	ids := usageUsers(t, db, 3)
	fail := true
	if err := db.Callback().Raw().Before("gorm:raw").Register("test:fail_usage", func(tx *gorm.DB) {
		if fail && strings.Contains(tx.Statement.SQL.String(), "UPDATE user_traffic") {
			tx.AddError(fmt.Errorf("injected failure"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	s := &Service{db: db}
	now := time.Now().UTC()
	s.recordUserUsage(&node, []NodeUserTraffic{{Username: itoa(ids[0]), Uplink: 100}, {Username: itoa(ids[1]), Downlink: 50}}, now)
	if got := usedBytes(db, ids); got != 0 {
		t.Fatalf("failed poll wrote %d bytes", got)
	}
	fail = false
	s.recordUserUsage(&node, []NodeUserTraffic{{Username: itoa(ids[0]), Uplink: 1}, {Username: itoa(ids[2]), Uplink: 7}}, now)
	if got := usedBytes(db, ids); got != 158 {
		t.Fatalf("used bytes = %d, want 158 (failed poll kept and retried)", got)
	}
	var history int64
	db.Model(&database.NodesUserUsageHistory{}).Where("user_id IN ?", ids).Select("COALESCE(SUM(total_bytes), 0)").Scan(&history)
	if history != 158 {
		t.Fatalf("history = %d, want 158", history)
	}
	if len(s.pendingUsage[node.UUID]) != 0 {
		t.Fatalf("pending usage left: %v", s.pendingUsage)
	}
}

func TestRecordUserUsageDropsPendingBytesOfDeletedUsers(t *testing.T) {
	db := usageDB(t)
	node := database.Node{UUID: uuid.NewString(), Name: "n1", Address: "192.0.2.3", ConsumptionMultiplier: 1_000_000_000}
	db.Create(&node)
	ids := usageUsers(t, db, 2)
	s := &Service{db: db}
	s.keepPendingUsage(node.UUID, []userUsage{{id: ids[0], raw: 40}, {id: ids[1], raw: 2}})
	if err := db.Exec("DELETE FROM users WHERE id = ?", ids[1]).Error; err != nil {
		t.Fatal(err)
	}
	s.recordUserUsage(&node, nil, time.Now().UTC())
	if got := usedBytes(db, ids[:1]); got != 40 {
		t.Fatalf("used bytes = %d, want 40", got)
	}
	if len(s.pendingUsage[node.UUID]) != 0 {
		t.Fatalf("bytes of a deleted user are retried forever: %v", s.pendingUsage)
	}
}

// Nodes report overlapping users concurrently. On PostgreSQL this used to
// deadlock and drop whole polls; every reported byte must be billed.
func TestRecordUserUsageConcurrentNodesBillEveryByte(t *testing.T) {
	db := usageDB(t)
	ids := usageUsers(t, db, 120)
	const nodeCount, polls = 6, 15
	var expected int64
	var mu sync.Mutex
	var wg sync.WaitGroup
	s := &Service{db: db}
	nodes := map[string]*database.Node{}
	for n := 0; n < nodeCount; n++ {
		node := &database.Node{UUID: uuid.NewString(), Name: fmt.Sprintf("n%d", n), Address: fmt.Sprintf("192.0.2.%d", 50+n), ConsumptionMultiplier: 1_000_000_000}
		if err := db.Create(node).Error; err != nil {
			t.Fatal(err)
		}
		nodes[node.UUID] = node
		wg.Add(1)
		go func(seed int) {
			defer wg.Done()
			rng := rand.New(rand.NewPCG(uint64(seed), 1))
			for p := 0; p < polls; p++ {
				stats := make([]NodeUserTraffic, 0, len(ids))
				var sum int64
				for _, i := range rng.Perm(len(ids)) {
					bytes := int64(rng.IntN(1000) + 1)
					stats = append(stats, NodeUserTraffic{Username: itoa(ids[i]), Uplink: bytes})
					sum += bytes
				}
				s.recordUserUsage(node, stats, time.Now().UTC())
				mu.Lock()
				expected += sum
				mu.Unlock()
			}
		}(n)
	}
	wg.Wait()
	for attempt := 0; len(s.pendingUsage) > 0; attempt++ {
		if attempt == 5 {
			t.Fatalf("usage still pending after retries: %d nodes", len(s.pendingUsage))
		}
		for nodeUUID := range s.pendingUsage {
			s.recordUserUsage(nodes[nodeUUID], nil, time.Now().UTC())
		}
	}
	if got := usedBytes(db, ids); got != expected {
		t.Fatalf("billed %d bytes, nodes reported %d", got, expected)
	}
}
