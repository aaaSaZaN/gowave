package nodes

import (
	"remnawave-go/internal/database"
	"sync"
	"testing"
)

func TestOriginalNodeHashedSetCompatibility(t *testing.T) {
	for _, fixture := range []struct {
		ids  []string
		hash string
	}{{nil, "0000000000000000"}, {[]string{"11111111-1111-1111-1111-111111111111"}, "-157b19e7-1e8505d1"}, {[]string{"a", "b", "a"}, "0000000100000025"}, {[]string{"emoji😀", "тест"}, "16282429-2e8ec093"}} {
		if actual := inboundUsersHash(fixture.ids); actual != fixture.hash {
			t.Fatalf("%v: %s != %s", fixture.ids, actual, fixture.hash)
		}
	}
}
func TestMetricsSnapshotsAreIsolated(t *testing.T) {
	db, err := database.Init("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	sql, _ := db.DB()
	defer sql.Close()
	database.AutoMigrate(db)
	service := NewService(db, nil)
	service.SetNodeMetrics("node", &NodeHotMetrics{OnlineUsers: 1, Versions: &NodeVersionsResponse{Node: "original"}})
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				snapshot := service.GetNodeMetrics("node")
				snapshot.Versions.Node = "local"
				service.UpdateNodeMetrics("node", func(m *NodeHotMetrics) { m.OnlineUsers = j })
			}
		}()
	}
	wg.Wait()
	if service.GetNodeMetrics("node").Versions.Node != "original" {
		t.Fatal("reader mutated shared metrics")
	}
}
