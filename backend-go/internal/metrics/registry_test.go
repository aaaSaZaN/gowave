package metrics

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
)

// testdata/prom-vectors.json comes from scripts/subscription-diff/prom-vectors.mjs:
// upstream's METRIC_PROVIDERS on prom-client 15.
func TestTextMatchesPromClient(t *testing.T) {
	data, err := os.ReadFile("testdata/prom-vectors.json")
	if err != nil {
		t.Fatal(err)
	}
	var vectors struct {
		Ops         [][]json.RawMessage
		Snapshots   []string
		ContentType string
	}
	if err := json.Unmarshal(data, &vectors); err != nil {
		t.Fatal(err)
	}
	if vectors.ContentType != ContentType {
		t.Errorf("content type %q", vectors.ContentType)
	}
	for _, metric := range Default.metrics {
		metric.Reset()
	}
	labelsOf := func(raw json.RawMessage) []Label {
		// Keep the object's key order.
		var labels []Label
		decoder := json.NewDecoder(bytes.NewReader(raw))
		_, _ = decoder.Token()
		for decoder.More() {
			key, _ := decoder.Token()
			value, _ := decoder.Token()
			labels = append(labels, Label{key.(string), value.(string)})
		}
		return labels
	}
	snapshot := 0
	for _, op := range vectors.Ops {
		var kind, name string
		json.Unmarshal(op[0], &kind)
		if len(op) > 1 {
			json.Unmarshal(op[1], &name)
		}
		metric := Default.byName[name]
		switch kind {
		case "snapshot":
			if got := Default.Text(); got != vectors.Snapshots[snapshot] {
				t.Errorf("snapshot %d:\n got: %q\nwant: %q", snapshot, got, vectors.Snapshots[snapshot])
			}
			snapshot++
		case "set", "inc":
			var value float64
			json.Unmarshal(op[3], &value)
			if kind == "set" {
				metric.Set(value, labelsOf(op[2]))
			} else {
				metric.Inc(value, labelsOf(op[2]))
			}
		case "remove":
			metric.Remove(labelsOf(op[2]))
		case "reset":
			metric.Reset()
		}
	}
}
