package responserules

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestHeadersCanonicalAndLegacy(t *testing.T) {
	for _, tc := range []struct {
		name, input, want string
	}{
		{"object", `{"x-first":"a","x-second":"b"}`, `{"x-first":"a","x-second":"b"}`},
		{"legacy", `[{"key":"x-first","value":"a"},{"key":"x-second","value":"b"}]`, `{"x-first":"a","x-second":"b"}`},
		{"duplicates", `[{"key":"X-Name","value":"1"},{"key":"x-name","value":"2"},{"key":"X-Name","value":"3"}]`, `{"X-Name":"3"}`},
		{"empty object", `{}`, `{}`},
		{"empty legacy", `[]`, `{}`},
		{"null compatibility", `null`, `{}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var headers Headers
			if err := json.Unmarshal([]byte(tc.input), &headers); err != nil {
				t.Fatal(err)
			}
			data, err := json.Marshal(headers)
			if err != nil || string(data) != tc.want {
				t.Fatalf("got %s (%v), want %s", data, err, tc.want)
			}
		})
	}
}

func TestHeadersRejectInvalidShapes(t *testing.T) {
	for _, input := range []string{`{"x":1}`, `"header"`, `true`, `[null]`, `[{}]`, `[{"key":"x","value":1}]`, `["x"]`} {
		var headers Headers
		if err := json.Unmarshal([]byte(input), &headers); err == nil {
			t.Errorf("accepted invalid headers: %s", input)
		}
	}
}

func TestNormalizePreservesRuleData(t *testing.T) {
	input := `{"version":"1","extra":9007199254740993,"rules":[{"name":"legacy","future":{"enabled":true},"responseModifications":{"headers":[{"key":"x-name","value":"{{USERNAME}}"}],"encryption":{"method":"age1","key":"secret"}}},{"responseModifications":{"headers":[]}},{"responseModifications":{"headers":{"x-existing":"value"}}},{"responseModifications":null},{}]}`
	want := strings.ReplaceAll(input, `[{"key":"x-name","value":"{{USERNAME}}"}]`, `{"x-name":"{{USERNAME}}"}`)
	want = strings.ReplaceAll(want, `"headers":[]`, `"headers":{}`)
	data, err := Normalize([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	var gotValue, wantValue any
	gotDecoder := json.NewDecoder(strings.NewReader(string(data)))
	gotDecoder.UseNumber()
	wantDecoder := json.NewDecoder(strings.NewReader(want))
	wantDecoder.UseNumber()
	if err := gotDecoder.Decode(&gotValue); err != nil {
		t.Fatal(err)
	}
	if err := wantDecoder.Decode(&wantValue); err != nil {
		t.Fatal(err)
	}
	gotJSON, _ := json.Marshal(gotValue)
	wantJSON, _ := json.Marshal(wantValue)
	if string(gotJSON) != string(wantJSON) {
		t.Fatalf("got %s, want %s", gotJSON, wantJSON)
	}
	replayed, err := Normalize(data)
	if err != nil || string(replayed) != string(data) {
		t.Fatalf("normalization is not idempotent: %s (%v)", replayed, err)
	}
}

func TestNormalizeUnchangedAndInvalidDocuments(t *testing.T) {
	for _, input := range []string{`{}`, `null`, `{"rules":[]}`, `{"rules":[{"responseModifications":{"headers":null}}]}`, ` {"rules":[{"responseModifications":{"headers":{"X-Test":"v"}}}]} `} {
		data, err := Normalize([]byte(input))
		if err != nil || string(data) != input {
			t.Errorf("changed %s into %s (%v)", input, data, err)
		}
	}
	for _, input := range []string{`{`, `[]`, `{"rules":{}}`, `{"rules":[{"responseModifications":{"headers":[{}]}}]}`, `{"rules":[{"responseModifications":{"headers":{"x":42}}}]}`} {
		if _, err := Normalize([]byte(input)); err == nil {
			t.Errorf("accepted invalid document: %s", input)
		}
	}
}
