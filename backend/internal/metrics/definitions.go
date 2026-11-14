package metrics

// Default is the registry served on METRICS_PORT; metrics are registered in
// METRIC_PROVIDERS order.
var Default = newRegistry([]Label{{"app", "remnawave"}})

const prefix = "remnawave"

func gauge(name, help string) *Metric   { return Default.register(prefix, name, help, "gauge") }
func counter(name, help string) *Metric { return Default.register(prefix, name, help, "counter") }

var (
	NodeOnlineUsers  = gauge("node_online_users", "Number of online users on a node")
	NodeStatus       = gauge("node_status", "Node connection status (1 - connected, 0 - disconnected)")
	UsersStatus      = gauge("users_status", "Counter for users statuses, updated every 1 minute")
	UsersOnlineStats = gauge("users_online_stats", "Counter for online stats of distinct users, updated every 1 minute")
	UsersTotal       = gauge("users_total", "Total number of users, updated every 1 minute")

	NodeInboundUploadBytes    = counter("node_inbound_upload_bytes", "Inbound upload bytes, updated every 30 seconds")
	NodeInboundDownloadBytes  = counter("node_inbound_download_bytes", "Inbound download bytes, updated every 30 seconds")
	NodeOutboundUploadBytes   = counter("node_outbound_upload_bytes", "Outbound upload bytes, updated every 30 seconds")
	NodeOutboundDownloadBytes = counter("node_outbound_download_bytes", "Outbound download bytes, updated every 30 seconds")

	ProcessRssBytes          = gauge("process_rss_bytes", "Process resident set size in bytes.")
	ProcessHeapUsedBytes     = gauge("process_heap_used_bytes", "Process heap used in bytes.")
	ProcessHeapTotalBytes    = gauge("process_heap_total_bytes", "Process total heap size in bytes.")
	ProcessExternalBytes     = gauge("process_external_bytes", "Process external memory in bytes (C++ objects bound to JS).")
	ProcessArrayBuffersBytes = gauge("process_array_buffers_bytes", "Process ArrayBuffers memory in bytes.")
	ProcessEventLoopDelayMs  = gauge("process_event_loop_delay_ms", "Mean event loop delay in milliseconds.")
	ProcessEventLoopP99Ms    = gauge("process_event_loop_p99_ms", "Event loop delay p99 in milliseconds.")
	ProcessActiveHandles     = gauge("process_active_handles", "Number of active resource handles.")
	ProcessUptimeSeconds     = gauge("process_uptime_seconds", "Process uptime in seconds.")

	NodeNetworkRxBytesPerSec = gauge("node_network_rx_bytes_per_sec", "Node network receive speed in bytes per second")
	NodeNetworkTxBytesPerSec = gauge("node_network_tx_bytes_per_sec", "Node network transmit speed in bytes per second")
	NodeNetworkRxBytesTotal  = gauge("node_network_rx_bytes_total", "Node network total received bytes since boot from default interface")
	NodeNetworkTxBytesTotal  = gauge("node_network_tx_bytes_total", "Node network total transmitted bytes since boot from default interface")

	NodeMemoryTotalBytes = gauge("node_memory_total_bytes", "Node total memory in bytes")
	NodeMemoryFreeBytes  = gauge("node_memory_free_bytes", "Node free memory in bytes")
	NodeUptimeSeconds    = gauge("node_uptime_seconds", "Node OS uptime in seconds")
	NodeCPUCount         = gauge("node_cpu_count", "Node CPU count")

	NodeCPULoadAvg1m  = gauge("node_cpu_load_avg_1m", "Node CPU load average 1 minute")
	NodeCPULoadAvg5m  = gauge("node_cpu_load_avg_5m", "Node CPU load average 5 minutes")
	NodeCPULoadAvg15m = gauge("node_cpu_load_avg_15m", "Node CPU load average 15 minutes")

	NodeSystemInfo = gauge("node_system_info", "Node system info")
	NodeBasicInfo  = gauge("node_basic_info", "Node basic info")
)

// NodeTraffic is one inbound/outbound entry of a node's combined stats.
type NodeTraffic struct {
	Tag              string
	Uplink, Downlink float64
}

// RecordNodeTraffic ports NodeMetricsSubscriber.handleNodeMetricsMessage.
func RecordNodeTraffic(nodeUUID string, inbounds, outbounds []NodeTraffic) {
	for _, inbound := range inbounds {
		labels := L("node_uuid", nodeUUID, "tag", inbound.Tag)
		NodeInboundUploadBytes.Inc(inbound.Uplink, labels)
		NodeInboundDownloadBytes.Inc(inbound.Downlink, labels)
	}
	for _, outbound := range outbounds {
		labels := L("node_uuid", nodeUUID, "tag", outbound.Tag)
		NodeOutboundUploadBytes.Inc(outbound.Uplink, labels)
		NodeOutboundDownloadBytes.Inc(outbound.Downlink, labels)
	}
}
