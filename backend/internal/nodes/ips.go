package nodes

import (
	"encoding/json"
	"fmt"
	"net/netip"
)

// NodeIP is one entry of a node's address inventory.
type NodeIP struct {
	IP     string `json:"ip"`
	Status string `json:"status"`
}

var nodeIPStatuses = map[string]bool{
	"INBOUND": true, "OUTBOUND": true, "MANAGEMENT": true, "TRANSIT": true, "MONITORING": true,
	"RESERVE": true, "BLOCKED": true, "FLAGGED": true, "DEPRECATED": true, "UNKNOWN": true,
}

// encodeNodeIPs validates the contract shape and returns the stored JSON.
func encodeNodeIPs(ips []NodeIP) (string, error) {
	if len(ips) > 64 {
		return "", fmt.Errorf("ips: at most 64 entries")
	}
	for i, entry := range ips {
		if _, err := netip.ParseAddr(entry.IP); err != nil {
			return "", fmt.Errorf("ips.%d.ip: invalid IP address", i)
		}
		if !nodeIPStatuses[entry.Status] {
			return "", fmt.Errorf("ips.%d.status: invalid status", i)
		}
	}
	if ips == nil {
		ips = []NodeIP{}
	}
	data, err := json.Marshal(ips)
	return string(data), err
}

// decodeNodeIPs reads stored IPs; plain strings from older Gowave builds become
// UNKNOWN entries.
func decodeNodeIPs(raw string) []NodeIP {
	result := []NodeIP{}
	if raw == "" {
		return result
	}
	if json.Unmarshal([]byte(raw), &result) == nil {
		return result
	}
	var legacy []string
	if json.Unmarshal([]byte(raw), &legacy) == nil {
		for _, ip := range legacy {
			result = append(result, NodeIP{IP: ip, Status: "UNKNOWN"})
		}
	}
	return result
}
