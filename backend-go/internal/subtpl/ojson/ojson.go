// Package ojson is an insertion-ordered JSON model with JavaScript semantics,
// so generated configs serialize exactly like upstream's JSON.stringify.
//
// Values are nil, bool, float64, string, []any or *Object. Undefined (a key
// JavaScript would drop) is represented by Undefined.
package ojson

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Object is a JSON object that remembers key insertion order.
type Object struct {
	keys   []string
	values map[string]any
}

type undefined struct{}

// Undefined marks a property JSON.stringify omits.
var Undefined = undefined{}

// New builds an object from alternating key, value pairs.
func New(pairs ...any) *Object {
	o := &Object{values: map[string]any{}}
	for i := 0; i+1 < len(pairs); i += 2 {
		o.Set(pairs[i].(string), pairs[i+1])
	}
	return o
}

func (o *Object) init() {
	if o.values == nil {
		o.values = map[string]any{}
	}
}

// Set assigns key; an existing key keeps its position, like JS assignment.
func (o *Object) Set(key string, value any) *Object {
	o.init()
	if _, ok := o.values[key]; !ok {
		o.keys = append(o.keys, key)
	}
	o.values[key] = value
	return o
}

// Get returns the value and whether the key exists.
func (o *Object) Get(key string) (any, bool) {
	if o == nil {
		return nil, false
	}
	value, ok := o.values[key]
	return value, ok
}

// Value returns the value or nil.
func (o *Object) Value(key string) any {
	value, _ := o.Get(key)
	return value
}

// Has reports whether key exists (even with a null/undefined value).
func (o *Object) Has(key string) bool {
	_, ok := o.Get(key)
	return ok
}

// Delete removes key.
func (o *Object) Delete(key string) {
	if o == nil || !o.Has(key) {
		return
	}
	delete(o.values, key)
	for i, k := range o.keys {
		if k == key {
			o.keys = append(o.keys[:i:i], o.keys[i+1:]...)
			break
		}
	}
}

// Keys returns keys in insertion order (JS also lists integer-like keys
// first; templates never use them).
func (o *Object) Keys() []string {
	if o == nil {
		return nil
	}
	return append([]string(nil), o.keys...)
}

// Len is the number of keys.
func (o *Object) Len() int {
	if o == nil {
		return 0
	}
	return len(o.keys)
}

// Spread copies src's keys into o, like {...o, ...src}.
func (o *Object) Spread(src *Object) *Object {
	if src == nil {
		return o
	}
	for _, key := range src.keys {
		o.Set(key, src.values[key])
	}
	return o
}

// Clone deep-copies any JSON value.
func Clone(value any) any {
	switch v := value.(type) {
	case *Object:
		if v == nil {
			return (*Object)(nil)
		}
		c := &Object{keys: append([]string(nil), v.keys...), values: make(map[string]any, len(v.values))}
		for k, item := range v.values {
			c.values[k] = Clone(item)
		}
		return c
	case []any:
		c := make([]any, len(v))
		for i, item := range v {
			c[i] = Clone(item)
		}
		return c
	}
	return value
}

// Parse decodes JSON preserving key order; numbers become float64.
func Parse(data []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	value, err := parseValue(dec)
	if err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, errors.New("ojson: trailing data")
	}
	return value, nil
}

// MustParse panics on invalid input; for constants.
func MustParse(text string) any {
	value, err := Parse([]byte(text))
	if err != nil {
		panic(err)
	}
	return value
}

func parseValue(dec *json.Decoder) (any, error) {
	token, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch t := token.(type) {
	case json.Delim:
		switch t {
		case '{':
			o := New()
			for dec.More() {
				keyToken, err := dec.Token()
				if err != nil {
					return nil, err
				}
				value, err := parseValue(dec)
				if err != nil {
					return nil, err
				}
				o.Set(keyToken.(string), value)
			}
			_, err := dec.Token()
			return o, err
		case '[':
			list := []any{}
			for dec.More() {
				value, err := parseValue(dec)
				if err != nil {
					return nil, err
				}
				list = append(list, value)
			}
			_, err := dec.Token()
			return list, err
		}
	case json.Number:
		return t.Float64()
	}
	return token, nil
}

// FromGo converts encoding/json-compatible Go data (maps sorted by key) into
// ojson values.
func FromGo(value any) any {
	data, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	parsed, _ := Parse(data)
	return parsed
}

// Stringify mirrors JSON.stringify(value) (indent 0).
func Stringify(value any) string {
	var b strings.Builder
	writeValue(&b, value, "", "")
	return b.String()
}

// MarshalJSON lets encoding/json embed an Object with its key order.
func (o *Object) MarshalJSON() ([]byte, error) {
	if o == nil {
		return []byte("null"), nil
	}
	return []byte(Stringify(o)), nil
}

// StringifyIndent mirrors JSON.stringify(value, null, indent).
func StringifyIndent(value any, indent int) string {
	var b strings.Builder
	writeValue(&b, value, strings.Repeat(" ", indent), "")
	return b.String()
}

// FormatNumber renders a float64 like JavaScript's Number#toString.
func FormatNumber(f float64) string {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return "null"
	}
	if f == 0 {
		return "0"
	}
	abs := math.Abs(f)
	if abs >= 1e21 || abs < 1e-6 {
		s := strconv.FormatFloat(f, 'e', -1, 64)
		// Go prints e+21 / e-07; JS prints e+21 / e-7.
		mantissa, exponent, _ := strings.Cut(s, "e")
		sign := exponent[0]
		digits := strings.TrimLeft(exponent[1:], "0")
		return mantissa + "e" + string(sign) + digits
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}

func writeValue(b *strings.Builder, value any, indent, prefix string) bool {
	switch v := value.(type) {
	case nil:
		b.WriteString("null")
	case undefined:
		return false
	case bool:
		if v {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
	case float64:
		b.WriteString(FormatNumber(v))
	case int:
		b.WriteString(strconv.Itoa(v))
	case int64:
		b.WriteString(strconv.FormatInt(v, 10))
	case uint64:
		b.WriteString(strconv.FormatUint(v, 10))
	case string:
		writeString(b, v)
	case []any:
		if len(v) == 0 {
			b.WriteString("[]")
			return true
		}
		inner := prefix + indent
		b.WriteByte('[')
		for i, item := range v {
			if i > 0 {
				b.WriteByte(',')
			}
			if indent != "" {
				b.WriteByte('\n')
				b.WriteString(inner)
			}
			if !writeValue(b, item, indent, inner) {
				b.WriteString("null")
			}
		}
		if indent != "" {
			b.WriteByte('\n')
			b.WriteString(prefix)
		}
		b.WriteByte(']')
	case []string:
		list := make([]any, len(v))
		for i, s := range v {
			list[i] = s
		}
		return writeValue(b, list, indent, prefix)
	case *Object:
		if v == nil {
			b.WriteString("null")
			return true
		}
		inner := prefix + indent
		wrote := false
		b.WriteByte('{')
		for _, key := range v.keys {
			item := v.values[key]
			if _, skip := item.(undefined); skip {
				continue
			}
			if wrote {
				b.WriteByte(',')
			}
			if indent != "" {
				b.WriteByte('\n')
				b.WriteString(inner)
			}
			writeString(b, key)
			b.WriteByte(':')
			if indent != "" {
				b.WriteByte(' ')
			}
			writeValue(b, item, indent, inner)
			wrote = true
		}
		if wrote && indent != "" {
			b.WriteByte('\n')
			b.WriteString(prefix)
		}
		b.WriteByte('}')
	default:
		return writeValue(b, FromGo(v), indent, prefix)
	}
	return true
}

// writeString quotes like JSON.stringify: only ", \, control characters and
// lone surrogates are escaped; HTML characters and U+2028 stay literal.
func writeString(b *strings.Builder, s string) {
	b.WriteByte('"')
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		switch {
		case r == '"':
			b.WriteString(`\"`)
		case r == '\\':
			b.WriteString(`\\`)
		case r == '\b':
			b.WriteString(`\b`)
		case r == '\f':
			b.WriteString(`\f`)
		case r == '\n':
			b.WriteString(`\n`)
		case r == '\r':
			b.WriteString(`\r`)
		case r == '\t':
			b.WriteString(`\t`)
		case r < 0x20:
			fmt.Fprintf(b, `\u%04x`, r)
		case r == utf8.RuneError && size == 1:
			fmt.Fprintf(b, `\u%04x`, s[i])
		default:
			b.WriteString(s[i : i+size])
		}
		i += size
	}
	b.WriteByte('"')
}

// IsNonEmptyObject reports an object with at least one key.
func IsNonEmptyObject(value any) bool {
	o, ok := value.(*Object)
	return ok && o != nil && o.Len() > 0
}

// Truthy mirrors JavaScript truthiness for JSON values.
func Truthy(value any) bool {
	switch v := value.(type) {
	case nil, undefined:
		return false
	case bool:
		return v
	case float64:
		return v != 0 && !math.IsNaN(v)
	case string:
		return v != ""
	case *Object:
		return v != nil
	}
	return true
}
