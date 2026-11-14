package ojson

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// testdata/vectors.json is produced by lodash 4 and Node's JSON.stringify.
func TestMatchesLodashAndJSONStringify(t *testing.T) {
	data, err := os.ReadFile("testdata/vectors.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Op   string          `json:"op"`
		Path string          `json:"path"`
		Obj  json.RawMessage `json:"obj"`
		Out  json.RawMessage `json:"out"`
		Out2 string          `json:"out2"`
	}
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		switch c.Op {
		case "toPath":
			var want []string
			json.Unmarshal(c.Out, &want)
			if got := ToPath(c.Path); strings.Join(got, "|") != strings.Join(want, "|") {
				t.Errorf("ToPath(%q) = %q, want %q", c.Path, got, want)
			}
		case "set", "unset":
			var want string
			json.Unmarshal(c.Out, &want)
			obj, _ := Parse(c.Obj)
			if c.Op == "set" {
				obj = Set(obj, ToPath(c.Path), 7.0)
			} else {
				Unset(obj, ToPath(c.Path))
			}
			if got := Stringify(obj); got != want {
				t.Errorf("%s(%s, %q) = %s, want %s", c.Op, c.Obj, c.Path, got, want)
			}
		case "stringify":
			var want string
			json.Unmarshal(c.Out, &want)
			obj, _ := Parse(c.Obj)
			if got := Stringify(obj); got != want {
				t.Errorf("Stringify = %s, want %s", got, want)
			}
			if got := StringifyIndent(obj, 2); got != c.Out2 {
				t.Errorf("StringifyIndent = %s, want %s", got, c.Out2)
			}
		}
	}
}

func TestSpreadKeepsExistingPosition(t *testing.T) {
	base := MustParse(`{"log":{},"outbounds":[1],"dns":{}}`).(*Object)
	result := New().Spread(base).Set("outbounds", []any{2.0}).Set("remarks", "r")
	if got := Stringify(result); got != `{"log":{},"outbounds":[2],"dns":{},"remarks":"r"}` {
		t.Fatal(got)
	}
}
