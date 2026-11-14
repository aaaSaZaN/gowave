package configprofiles

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
)

// XrayConfigKeyOrder defines canonical order of root Xray keys matching xray-typed
var XrayConfigKeyOrder = []string{
	"snippets",
	"version",
	"log",
	"dns",
	"inbounds",
	"outbounds",
	"routing",
	"policy",
	"transport",
	"stats",
	"reverse",
	"fakeDns",
	"observatory",
	"burstObservatory",
	"api",
	"metrics",
}

// InboundKeyOrder ensures tag is first, followed by standard inbound properties
var InboundKeyOrder = []string{
	"tag",
	"port",
	"listen",
	"protocol",
	"settings",
	"streamSettings",
	"sniffing",
	"allocate",
}

// StreamSettingsKeyOrder defines canonical order for streamSettings
var StreamSettingsKeyOrder = []string{
	"network",
	"security",
	"tlsSettings",
	"realitySettings",
	"tcpSettings",
	"kcpSettings",
	"wsSettings",
	"httpSettings",
	"quicSettings",
	"dsSettings",
	"grpcSettings",
	"httpupgradeSettings",
	"xhttpSettings",
	"hy2Settings",
	"sockopt",
}

// OrderedObject preserves key insertion order
type OrderedObject struct {
	Keys   []string
	Values map[string]interface{}
}

func parseOrderedValue(dec *json.Decoder) (interface{}, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	delim, ok := tok.(json.Delim)
	if !ok {
		return tok, nil
	}
	if delim == '{' {
		obj := &OrderedObject{Values: make(map[string]interface{})}
		for dec.More() {
			keyTok, err := dec.Token()
			if err != nil {
				return nil, err
			}
			key, ok := keyTok.(string)
			if !ok {
				return nil, fmt.Errorf("expected string key, got %v", keyTok)
			}
			val, err := parseOrderedValue(dec)
			if err != nil {
				return nil, err
			}
			if _, exists := obj.Values[key]; !exists {
				obj.Keys = append(obj.Keys, key)
			}
			obj.Values[key] = val
		}
		// consume closing '}'
		_, err = dec.Token()
		return obj, err
	} else if delim == '[' {
		var list []interface{}
		for dec.More() {
			val, err := parseOrderedValue(dec)
			if err != nil {
				return nil, err
			}
			list = append(list, val)
		}
		// consume closing ']'
		_, err = dec.Token()
		return list, err
	}
	return nil, fmt.Errorf("unexpected delim %v", delim)
}

func reorderKeys(obj *OrderedObject, canonicalOrder []string) {
	if obj == nil {
		return
	}
	var newKeys []string
	seen := make(map[string]bool, len(obj.Keys))
	for _, k := range canonicalOrder {
		if _, ok := obj.Values[k]; ok {
			newKeys = append(newKeys, k)
			seen[k] = true
		}
	}
	for _, k := range obj.Keys {
		if !seen[k] {
			newKeys = append(newKeys, k)
		}
	}
	obj.Keys = newKeys
}

func sortInboundObject(ibObj *OrderedObject) {
	if ibObj == nil {
		return
	}
	reorderKeys(ibObj, InboundKeyOrder)
	if ss, ok := ibObj.Values["streamSettings"].(*OrderedObject); ok {
		reorderKeys(ss, StreamSettingsKeyOrder)
	}
}

// SortXrayConfig parses raw Xray JSON and returns canonically formatted and ordered JSON
func SortXrayConfig(data []byte) ([]byte, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return []byte("{}"), nil
	}

	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()

	val, err := parseOrderedValue(dec)
	if err != nil {
		return nil, fmt.Errorf("invalid json: %w", err)
	}

	root, ok := val.(*OrderedObject)
	if !ok {
		// If not an object, just return original data
		return data, nil
	}

	// 1. Reorder root keys
	reorderKeys(root, XrayConfigKeyOrder)

	// 2. Reorder inbounds
	if inbs, ok := root.Values["inbounds"].([]interface{}); ok {
		for _, ibElem := range inbs {
			if ibObj, ok := ibElem.(*OrderedObject); ok {
				sortInboundObject(ibObj)
			}
		}
	}

	var buf bytes.Buffer
	if err := encodeOrderedValue(&buf, root, ""); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// SortInboundConfig parses a single inbound JSON object and returns it with tag at the top
func SortInboundConfig(data []byte) ([]byte, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return []byte("{}"), nil
	}

	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()

	val, err := parseOrderedValue(dec)
	if err != nil {
		return nil, fmt.Errorf("invalid json: %w", err)
	}

	ibObj, ok := val.(*OrderedObject)
	if !ok {
		return data, nil
	}

	sortInboundObject(ibObj)

	var buf bytes.Buffer
	if err := encodeOrderedValue(&buf, ibObj, ""); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func encodeOrderedValue(w io.Writer, val interface{}, indent string) error {
	switch v := val.(type) {
	case *OrderedObject:
		if len(v.Keys) == 0 {
			w.Write([]byte("{}"))
			return nil
		}
		w.Write([]byte("{\n"))
		for i, k := range v.Keys {
			w.Write([]byte(indent + "  "))
			kb, _ := json.Marshal(k)
			w.Write(kb)
			w.Write([]byte(": "))
			if err := encodeOrderedValue(w, v.Values[k], indent+"  "); err != nil {
				return err
			}
			if i < len(v.Keys)-1 {
				w.Write([]byte(","))
			}
			w.Write([]byte("\n"))
		}
		w.Write([]byte(indent + "}"))
	case []interface{}:
		if len(v) == 0 {
			w.Write([]byte("[]"))
			return nil
		}
		w.Write([]byte("[\n"))
		for i, elem := range v {
			w.Write([]byte(indent + "  "))
			if err := encodeOrderedValue(w, elem, indent+"  "); err != nil {
				return err
			}
			if i < len(v)-1 {
				w.Write([]byte(","))
			}
			w.Write([]byte("\n"))
		}
		w.Write([]byte(indent + "]"))
	case json.Number:
		w.Write([]byte(v.String()))
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return err
		}
		w.Write(b)
	}
	return nil
}
