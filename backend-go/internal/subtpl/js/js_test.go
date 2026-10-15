package js

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
	"time"
)

// testdata/vectors.json comes from dayjs, xbytes, encodeURIComponent and
// Buffer in Node (TZ=UTC); see scripts/subscription-diff/js-vectors.cjs.
func TestMatchesNode(t *testing.T) {
	data, err := os.ReadFile("testdata/vectors.json")
	if err != nil {
		t.Fatal(err)
	}
	var v struct {
		Dayjs []struct{ Format, Out string }
		URI   []struct {
			In  string `json:"in"`
			Out string `json:"out"`
		}
		Base64 []struct {
			In  string `json:"in"`
			Out string `json:"out"`
		}
		Bytes []struct {
			In     float64 `json:"in"`
			Out    string  `json:"out"`
			Sticky string  `json:"sticky"`
		}
	}
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatal(err)
	}
	date := time.Date(2026, 3, 4, 15, 6, 7, 89e6, time.UTC)
	for _, c := range v.Dayjs {
		if got := DayjsFormat(date, c.Format); got != c.Out {
			t.Errorf("DayjsFormat(%q) = %q, want %q", c.Format, got, c.Out)
		}
	}
	for _, c := range v.URI {
		if got := EncodeURIComponent(c.In); got != c.Out {
			t.Errorf("EncodeURIComponent(%q) = %q, want %q", c.In, got, c.Out)
		}
	}
	for _, c := range v.Base64 {
		if got := hex.EncodeToString(DecodeBase64(c.In)); got != c.Out {
			t.Errorf("DecodeBase64(%q) = %s, want %s", c.In, got, c.Out)
		}
	}
	for _, c := range v.Bytes {
		if got := PrettyBytes(c.In, false, 0); got != c.Out {
			t.Errorf("PrettyBytes(%v) = %q, want %q", c.In, got, c.Out)
		}
		if got := PrettyBytes(c.In, true, 3); got != c.Sticky {
			t.Errorf("PrettyBytes(%v, sticky) = %q, want %q", c.In, got, c.Sticky)
		}
	}
}
