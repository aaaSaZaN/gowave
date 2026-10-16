package subtpl

import (
	"encoding/json"
	"os"
	"testing"

	"gowave/internal/subtpl/ojson"
)

// testdata/yaml-vectors.json comes from scripts/subscription-diff/yaml-vectors.mjs
// (js-yaml dump() as bundled with upstream).
func TestDumpYAMLMatchesJsYaml(t *testing.T) {
	data, err := os.ReadFile("testdata/yaml-vectors.json")
	if err != nil {
		t.Fatal(err)
	}
	var vectors []struct {
		Input  json.RawMessage
		Output string
	}
	if err := json.Unmarshal(data, &vectors); err != nil {
		t.Fatal(err)
	}
	for i, vector := range vectors {
		input, err := ojson.Parse(vector.Input)
		if err != nil {
			t.Fatal(err)
		}
		got, err := DumpYAML(input)
		if err != nil {
			t.Fatal(err)
		}
		if got != vector.Output {
			t.Errorf("vector %d %.80s\n got: %q\nwant: %q", i, vector.Input, got, vector.Output)
		}
	}
}
