// Package responserules handles the canonical Response Rules header format and
// compatibility with legacy upstream header arrays.
package responserules

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"gowave/internal/subtpl/ojson"
)

type Header struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Headers serializes as a name-to-value object. Arrays are accepted only for
// legacy compatibility. Order matters when differently cased names overlap.
type Headers []Header

func (h *Headers) UnmarshalJSON(data []byte) error {
	value, err := ojson.Parse(data)
	if err != nil {
		return err
	}
	var result Headers
	switch v := value.(type) {
	case *ojson.Object:
		result = make(Headers, 0, v.Len())
		for _, key := range v.Keys() {
			value, ok := v.Value(key).(string)
			if !ok {
				return fmt.Errorf("header %q must have a string value", key)
			}
			result = append(result, Header{Key: key, Value: value})
		}
	case []any:
		result = make(Headers, 0, len(v))
		for _, item := range v {
			entry, ok := item.(*ojson.Object)
			if !ok {
				return fmt.Errorf("legacy headers must contain key/value objects")
			}
			key, keyOK := entry.Value("key").(string)
			value, valueOK := entry.Value("value").(string)
			if !keyOK || !valueOK {
				return fmt.Errorf("legacy headers require string key and value fields")
			}
			result = append(result, Header{Key: key, Value: value})
		}
	case nil:
	default:
		return fmt.Errorf("headers must be an object (legacy arrays are accepted for compatibility)")
	}
	*h = result
	return nil
}

func (h Headers) MarshalJSON() ([]byte, error) {
	object := ojson.New()
	seen := make(map[string]string, len(h))
	for _, header := range h {
		// HTTP header names are case-insensitive. Keep the last assignment,
		// including its position, so conversion preserves Header.Set behavior.
		lower := strings.ToLower(header.Key)
		if previous, ok := seen[lower]; ok {
			object.Delete(previous)
		}
		seen[lower] = header.Key
		object.Set(header.Key, header.Value)
	}
	return json.Marshal(object)
}

// Normalize converts legacy header arrays without decoding rules into a fixed
// struct, preserving unrelated fields and exact JSON numbers during upgrades.
// Documents with no legacy headers are returned unchanged.
func Normalize(data []byte) ([]byte, error) {
	var config map[string]json.RawMessage
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	if len(config["rules"]) == 0 {
		return data, nil
	}
	var rules []map[string]json.RawMessage
	if err := json.Unmarshal(config["rules"], &rules); err != nil {
		return nil, err
	}
	changed := false
	for _, rule := range rules {
		raw := rule["responseModifications"]
		if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			continue
		}
		var mods map[string]json.RawMessage
		if err := json.Unmarshal(raw, &mods); err != nil {
			return nil, err
		}
		rawHeaders := bytes.TrimSpace(mods["headers"])
		if len(rawHeaders) == 0 || bytes.Equal(rawHeaders, []byte("null")) {
			continue
		}
		var headers Headers
		if err := json.Unmarshal(rawHeaders, &headers); err != nil {
			return nil, fmt.Errorf("response rules headers: %w", err)
		}
		if rawHeaders[0] != '[' {
			continue
		}
		mods["headers"], _ = json.Marshal(headers)
		rule["responseModifications"], _ = json.Marshal(mods)
		changed = true
	}
	if !changed {
		return data, nil
	}
	config["rules"], _ = json.Marshal(rules)
	return json.Marshal(config)
}
