package system

import (
	"net/http"
	"sort"
	"strings"

	"gowave/internal/database"
	"gowave/internal/httpx"
	"gowave/internal/metrics"
	"gowave/internal/prometheus"
	"gowave/internal/subtpl/js"
	"gowave/internal/subtpl/ojson"
)

type trafficByTag struct {
	tags             []string
	upload, download map[string]float64
}

func (t *trafficByTag) add(tag string, value float64, upload bool) {
	if t.upload == nil {
		t.upload, t.download = map[string]float64{}, map[string]float64{}
	}
	if _, ok := t.upload[tag]; !ok {
		if _, ok := t.download[tag]; !ok {
			t.tags = append(t.tags, tag)
		}
	}
	if upload {
		t.upload[tag] = value
	} else {
		t.download[tag] = value
	}
}

func (t *trafficByTag) stats() []any {
	tags := append([]string(nil), t.tags...)
	sort.Slice(tags, func(i, j int) bool { return strings.Compare(tags[i], tags[j]) < 0 })
	out := make([]any, 0, len(tags))
	for _, tag := range tags {
		out = append(out, ojson.New("tag", tag,
			"upload", js.PrettyBytes(t.upload[tag], false, 0),
			"download", js.PrettyBytes(t.download[tag], false, 0)))
	}
	return out
}

// GetNodesMetrics ports SystemService.getNodesMetrics: the node series of
// the Prometheus registry grouped per node.
func (h *Handler) GetNodesMetrics(w http.ResponseWriter, r *http.Request) {
	var nodes []database.Node
	if err := h.db.Preload("Provider").Find(&nodes).Error; err != nil {
		httpx.Response(w, ojson.New("nodes", []any{}))
		return
	}
	known := map[string]*database.Node{}
	for i := range nodes {
		known[nodes[i].UUID] = &nodes[i]
	}
	type group struct {
		online              float64
		inbounds, outbounds trafficByTag
	}
	groups := map[string]*group{}
	var order []string
	groupFor := func(uuid string) *group {
		g, ok := groups[uuid]
		if !ok {
			g = &group{}
			groups[uuid] = g
			order = append(order, uuid)
		}
		return g
	}
	for _, s := range metrics.NodeOnlineUsers.Samples() {
		groupFor(s.Label("node_uuid")).online = s.Value
	}
	for _, source := range []struct {
		metric   *metrics.Metric
		inbound  bool
		isUpload bool
	}{
		{metrics.NodeInboundUploadBytes, true, true}, {metrics.NodeInboundDownloadBytes, true, false},
		{metrics.NodeOutboundUploadBytes, false, true}, {metrics.NodeOutboundDownloadBytes, false, false},
	} {
		for _, s := range source.metric.Samples() {
			g := groupFor(s.Label("node_uuid"))
			target := &g.outbounds
			if source.inbound {
				target = &g.inbounds
			}
			target.add(s.Label("tag"), s.Value, source.isUpload)
		}
	}
	result := []*ojson.Object{}
	positions := map[*ojson.Object]int{}
	for _, uuid := range order {
		node, ok := known[uuid]
		if !ok {
			continue
		}
		g := groups[uuid]
		if len(g.inbounds.tags) == 0 && len(g.outbounds.tags) == 0 {
			continue
		}
		provider := "unknown"
		if node.Provider != nil && node.Provider.Name != "" {
			provider = node.Provider.Name
		}
		item := ojson.New("nodeUuid", node.UUID, "nodeName", node.Name, "countryEmoji", prometheus.CountryEmoji(node.CountryCode),
			"providerName", provider, "usersOnline", g.online, "inboundsStats", g.inbounds.stats(), "outboundsStats", g.outbounds.stats())
		positions[item] = node.ViewPosition
		result = append(result, item)
	}
	sort.SliceStable(result, func(i, j int) bool { return positions[result[i]] < positions[result[j]] })
	list := make([]any, len(result))
	for i, item := range result {
		list[i] = item
	}
	httpx.Response(w, ojson.New("nodes", list))
}
