package subtpl

import (
	"encoding/json"
	"os"
	"testing"

	"gowave/internal/database"
	"gowave/internal/subtpl/ojson"
)

// The defaults Gowave seeds must equal upstream's DEFAULT_TEMPLATE_* (copied
// into testdata/matrix.json by scripts/subscription-diff/matrix.mjs).
func TestSeededTemplatesMatchUpstream(t *testing.T) {
	data, err := os.ReadFile("testdata/matrix.json")
	if err != nil {
		t.Fatal(err)
	}
	var matrix struct{ Templates map[string]json.RawMessage }
	if err := json.Unmarshal(data, &matrix); err != nil {
		t.Fatal(err)
	}
	seeded := map[string]string{
		"XRAY_JSON": database.DefaultTemplateXrayJson, "SINGBOX": database.DefaultTemplateSingbox,
		"MIHOMO": database.DefaultTemplateMihomo, "STASH": database.DefaultTemplateStash, "CLASH": database.DefaultTemplateClash,
	}
	for kind, text := range seeded {
		want, err := templateFrom(matrix.Templates[kind])
		if err != nil {
			t.Fatal(err)
		}
		var got *ojson.Object
		if kind == "XRAY_JSON" || kind == "SINGBOX" {
			value, err := ojson.Parse([]byte(text))
			if err != nil {
				t.Fatalf("%s: %v", kind, err)
			}
			got, _ = value.(*ojson.Object)
		} else {
			value, err := LoadYAML(text)
			if err != nil {
				t.Fatalf("%s: %v", kind, err)
			}
			got, _ = value.(*ojson.Object)
		}
		if ojson.StringifyIndent(got, 1) != ojson.StringifyIndent(want, 1) {
			t.Errorf("%s default template differs from upstream", kind)
			reportDiff(t, ojson.StringifyIndent(got, 1), ojson.StringifyIndent(want, 1))
		}
	}
}
