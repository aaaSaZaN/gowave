// Package prometheus ports upstream's scheduler metrics: ExportMetricsTask
// (every 15 seconds), SyncMetricsTask (every 6 hours) and the reporter
// served on METRICS_PORT.
package prometheus

import (
	"context"
	"log"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"

	"gowave/internal/database"
	"gowave/internal/metrics"
	"gowave/internal/nodes"
)

const (
	exportInterval = 15 * time.Second
	syncInterval   = 6 * time.Hour
	userStatsTTL   = time.Minute
)

// NodeStats gives the hot per-node data upstream keeps in Redis.
type NodeStats interface {
	GetNodeMetrics(uuid string) *nodes.NodeHotMetrics
}

// Exporter fills the metrics registry.
type Exporter struct {
	db        *gorm.DB
	nodes     NodeStats
	started   time.Time
	mu        sync.Mutex
	userStats *shortUserStats
	userAt    time.Time
}

func NewExporter(db *gorm.DB, nodeStats NodeStats) *Exporter {
	return &Exporter{db: db, nodes: nodeStats, started: time.Now()}
}

// Run exports until ctx ends.
func (e *Exporter) Run(ctx context.Context) {
	export := time.NewTicker(exportInterval)
	sync := time.NewTicker(syncInterval)
	defer export.Stop()
	defer sync.Stop()
	e.Export()
	for {
		select {
		case <-ctx.Done():
			return
		case <-export.C:
			e.Export()
		case <-sync.C:
			e.Sync()
		}
	}
}

// Export is ExportMetricsTask.handleCron.
func (e *Exporter) Export() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.reportShortUserStats()
	e.reportNodesStats()
	e.reportRuntimeMetrics()
}

type shortUserStats struct {
	statusCounts []int64
	totalUsers   int64
	onlineStats  [4]int64
}

var userStatuses = []string{"ACTIVE", "DISABLED", "LIMITED", "EXPIRED"}

var onlineMetricTypes = []string{"onlineNow", "lastDay", "lastWeek", "neverOnline"}

// loadShortUserStats ports GetShortUserStatsQuery (getUserStats and
// getUserOnlineStats).
func (e *Exporter) loadShortUserStats() (*shortUserStats, error) {
	var rows []struct {
		Status string
		Count  int64
	}
	if err := e.db.Model(&database.User{}).Select("status, COUNT(*) AS count").Where("status IN ?", userStatuses).Group("status").Scan(&rows).Error; err != nil {
		return nil, err
	}
	stats := &shortUserStats{statusCounts: make([]int64, len(userStatuses))}
	for i, status := range userStatuses {
		for _, row := range rows {
			if row.Status == status {
				stats.statusCounts[i] = row.Count
			}
		}
		stats.totalUsers += stats.statusCounts[i]
	}
	now := time.Now().UTC()
	var online struct{ OnlineNow, LastDay, LastWeek, NeverOnline int64 }
	if err := e.db.Raw(`SELECT
		COUNT(CASE WHEN online_at >= ? THEN 1 END) AS online_now,
		COUNT(CASE WHEN online_at >= ? THEN 1 END) AS last_day,
		COUNT(CASE WHEN online_at >= ? THEN 1 END) AS last_week,
		COUNT(CASE WHEN online_at IS NULL THEN 1 END) AS never_online
		FROM user_traffic`, now.Add(-30*time.Second), now.AddDate(0, 0, -1), now.AddDate(0, 0, -7)).Scan(&online).Error; err != nil {
		return nil, err
	}
	stats.onlineStats = [4]int64{online.OnlineNow, online.LastDay, online.LastWeek, online.NeverOnline}
	return stats, nil
}

func (e *Exporter) reportShortUserStats() {
	if e.userStats == nil || time.Since(e.userAt) > userStatsTTL {
		stats, err := e.loadShortUserStats()
		if err != nil {
			log.Printf("[METRICS] user stats: %v", err)
			return
		}
		e.userStats, e.userAt = stats, time.Now()
	}
	stats := e.userStats
	for i, status := range userStatuses {
		metrics.UsersStatus.Set(float64(stats.statusCounts[i]), metrics.L("status", status))
	}
	for i, metricType := range onlineMetricTypes {
		metrics.UsersOnlineStats.Set(float64(stats.onlineStats[i]), metrics.L("metricType", metricType))
	}
	metrics.UsersTotal.Set(float64(stats.totalUsers), metrics.L("type", "all"))
}

// CountryEmoji ports resolveCountryEmoji.
func CountryEmoji(code string) string {
	unknown := string([]rune{0x1F3F4, 0x200D, 0x2620, 0xFE0F})
	if len(code) != 2 || code == "XX" {
		return unknown
	}
	runes := make([]rune, 2)
	for i, c := range strings.ToUpper(code) {
		if c < 'A' || c > 'Z' {
			return unknown
		}
		runes[i] = c + 127397
	}
	return string(runes)
}

type nodeRow struct {
	UUID         string
	Name         string
	CountryCode  string
	IsConnected  bool
	Tags         database.StringArray
	ProviderName *string
}

func (e *Exporter) allNodes() ([]nodeRow, error) {
	var rows []nodeRow
	err := e.db.Table("nodes").
		Select("nodes.uuid, nodes.name, nodes.country_code, nodes.is_connected, nodes.tags, infra_providers.name AS provider_name").
		Joins("LEFT JOIN infra_providers ON infra_providers.uuid = nodes.provider_uuid").
		Order("nodes.view_position ASC").Scan(&rows).Error
	return rows, err
}

func basicLabels(node nodeRow) []metrics.Label {
	provider := "unknown"
	if node.ProviderName != nil && *node.ProviderName != "" {
		provider = *node.ProviderName
	}
	return metrics.L("node_uuid", node.UUID, "node_name", node.Name, "node_country_emoji", CountryEmoji(node.CountryCode),
		"provider_name", provider, "tags", strings.Join(node.Tags, ","))
}

var nodeSystemMetrics = []*metrics.Metric{
	metrics.NodeMemoryTotalBytes, metrics.NodeMemoryFreeBytes, metrics.NodeUptimeSeconds, metrics.NodeCPUCount,
	metrics.NodeNetworkRxBytesPerSec, metrics.NodeNetworkRxBytesTotal, metrics.NodeNetworkTxBytesPerSec, metrics.NodeNetworkTxBytesTotal,
	metrics.NodeCPULoadAvg1m, metrics.NodeCPULoadAvg5m, metrics.NodeCPULoadAvg15m,
}

func removeNodeSystemMetrics(labels []metrics.Label) {
	for _, metric := range nodeSystemMetrics {
		metric.Remove(labels)
	}
}

func loadAvg(values []float64, i int) float64 {
	if i < len(values) {
		return values[i]
	}
	return 0
}

// reportNodesStats ports ExportMetricsTask.reportNodesStats.
func (e *Exporter) reportNodesStats() {
	rows, err := e.allNodes()
	if err != nil {
		log.Printf("[METRICS] nodes: %v", err)
		return
	}
	metrics.NodeBasicInfo.Reset()
	metrics.NodeSystemInfo.Reset()
	for _, node := range rows {
		metrics.NodeBasicInfo.Set(1, basicLabels(node))
		base := metrics.L("node_uuid", node.UUID)
		status := 0.0
		if node.IsConnected {
			status = 1
		}
		metrics.NodeStatus.Set(status, base)

		var hot *nodes.NodeHotMetrics
		if e.nodes != nil {
			hot = e.nodes.GetNodeMetrics(node.UUID)
		}
		// Upstream's hot cache always yields an entry (0 online users).
		onlineUsers := 0
		if hot != nil {
			onlineUsers = hot.OnlineUsers
		}
		metrics.NodeOnlineUsers.Set(float64(onlineUsers), base)
		if hot == nil {
			removeNodeSystemMetrics(base)
			continue
		}
		system := hot.System
		if system == nil || system.System == nil || system.System.Info == nil || system.System.Stats == nil {
			removeNodeSystemMetrics(base)
			continue
		}
		info, stats := system.System.Info, system.System.Stats
		metrics.NodeSystemInfo.Set(1, metrics.L("node_uuid", node.UUID, "arch", info.Arch, "cpu_model", info.CPUModel,
			"hostname", info.Hostname, "platform", info.Platform, "release", info.Release, "version", info.Version))
		metrics.NodeMemoryTotalBytes.Set(float64(info.MemoryTotal), base)
		metrics.NodeMemoryFreeBytes.Set(float64(stats.MemoryFree), base)
		metrics.NodeUptimeSeconds.Set(stats.Uptime, base)
		metrics.NodeCPUCount.Set(float64(info.CPUs), base)
		metrics.NodeCPULoadAvg1m.Set(loadAvg(stats.LoadAvg, 0), base)
		metrics.NodeCPULoadAvg5m.Set(loadAvg(stats.LoadAvg, 1), base)
		metrics.NodeCPULoadAvg15m.Set(loadAvg(stats.LoadAvg, 2), base)
		if network := stats.Interface; network != nil {
			metrics.NodeNetworkRxBytesPerSec.Set(float64(network.RxBytesPerSec), base)
			metrics.NodeNetworkTxBytesPerSec.Set(float64(network.TxBytesPerSec), base)
			metrics.NodeNetworkRxBytesTotal.Set(float64(network.RxTotal), base)
			metrics.NodeNetworkTxBytesTotal.Set(float64(network.TxTotal), base)
		} else {
			metrics.NodeNetworkRxBytesPerSec.Remove(base)
			metrics.NodeNetworkTxBytesPerSec.Remove(base)
			metrics.NodeNetworkRxBytesTotal.Remove(base)
			metrics.NodeNetworkTxBytesTotal.Remove(base)
		}
	}
}

// RuntimeMetric is upstream's RuntimeMetric for this process. Go has no event
// loop: those fields report 0.
type RuntimeMetric struct {
	Rss              uint64  `json:"rss"`
	HeapUsed         uint64  `json:"heapUsed"`
	HeapTotal        uint64  `json:"heapTotal"`
	External         uint64  `json:"external"`
	ArrayBuffers     uint64  `json:"arrayBuffers"`
	EventLoopDelayMs float64 `json:"eventLoopDelayMs"`
	EventLoopP99Ms   float64 `json:"eventLoopP99Ms"`
	ActiveHandles    int     `json:"activeHandles"`
	Uptime           float64 `json:"uptime"`
	Pid              int     `json:"pid"`
	Timestamp        int64   `json:"timestamp"`
	InstanceID       string  `json:"instanceId"`
	InstanceType     string  `json:"instanceType"`
}

var processStart = time.Now()

// CurrentRuntimeMetric ports RuntimeMetricsService.collect for this process.
func CurrentRuntimeMetric() RuntimeMetric {
	var memory runtime.MemStats
	runtime.ReadMemStats(&memory)
	instanceID := os.Getenv("INSTANCE_ID")
	if instanceID == "" {
		instanceID = "0"
	}
	instanceType := os.Getenv("INSTANCE_TYPE")
	if instanceType == "" {
		instanceType = "api"
	}
	return RuntimeMetric{
		Rss: residentSetSize(memory.Sys), HeapUsed: memory.HeapAlloc, HeapTotal: memory.HeapSys,
		ActiveHandles: runtime.NumGoroutine(), Uptime: time.Since(processStart).Seconds(), Pid: os.Getpid(),
		Timestamp: time.Now().UnixMilli(), InstanceID: instanceID, InstanceType: instanceType,
	}
}

func (e *Exporter) reportRuntimeMetrics() {
	m := CurrentRuntimeMetric()
	labels := metrics.L("instance_id", m.InstanceID, "instance_name", m.InstanceType)
	metrics.ProcessRssBytes.Set(float64(m.Rss), labels)
	metrics.ProcessHeapUsedBytes.Set(float64(m.HeapUsed), labels)
	metrics.ProcessHeapTotalBytes.Set(float64(m.HeapTotal), labels)
	metrics.ProcessExternalBytes.Set(float64(m.External), labels)
	metrics.ProcessArrayBuffersBytes.Set(float64(m.ArrayBuffers), labels)
	metrics.ProcessEventLoopDelayMs.Set(m.EventLoopDelayMs, labels)
	metrics.ProcessEventLoopP99Ms.Set(m.EventLoopP99Ms, labels)
	metrics.ProcessActiveHandles.Set(float64(m.ActiveHandles), labels)
	metrics.ProcessUptimeSeconds.Set(m.Uptime, labels)
}

var nodeScopedMetrics = []*metrics.Metric{
	metrics.NodeOnlineUsers, metrics.NodeStatus, metrics.NodeMemoryTotalBytes, metrics.NodeMemoryFreeBytes,
	metrics.NodeUptimeSeconds, metrics.NodeCPUCount, metrics.NodeNetworkRxBytesPerSec, metrics.NodeNetworkTxBytesPerSec,
	metrics.NodeNetworkRxBytesTotal, metrics.NodeNetworkTxBytesTotal, metrics.NodeBasicInfo, metrics.NodeSystemInfo,
	metrics.NodeCPULoadAvg1m, metrics.NodeCPULoadAvg5m, metrics.NodeCPULoadAvg15m,
	metrics.NodeInboundUploadBytes, metrics.NodeInboundDownloadBytes, metrics.NodeOutboundUploadBytes, metrics.NodeOutboundDownloadBytes,
}

// Sync ports SyncMetricsTask: series of deleted nodes are dropped.
func (e *Exporter) Sync() {
	e.mu.Lock()
	defer e.mu.Unlock()
	rows, err := e.allNodes()
	if err != nil || len(rows) == 0 {
		return
	}
	known := map[string]bool{}
	for _, node := range rows {
		known[node.UUID] = true
	}
	for _, metric := range nodeScopedMetrics {
		for _, sample := range metric.Samples() {
			if !known[sample.Label("node_uuid")] {
				metric.Remove(sample.Labels)
			}
		}
	}
}
