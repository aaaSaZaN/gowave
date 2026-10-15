package ojson

import (
	"regexp"
	"strconv"
	"strings"
)

var (
	propertyName = regexp.MustCompile(`[^.[\]]+|\[(?:([^"'][^[]*)|(["'])((?:(?:[^\\]|\\.)*?))(["']))\]|(?:\.|\[\])(?:\.|\[\]|$)`)
	indexKey     = regexp.MustCompile(`^(?:0|[1-9]\d*)$`)
	escapeChar   = regexp.MustCompile(`\\(\\)?`)
)

// ToPath ports lodash's stringToPath: "a.b[0]['c.d']" → [a b 0 c.d].
func ToPath(path string) []string {
	var result []string
	if strings.HasPrefix(path, ".") {
		result = append(result, "")
	}
	for _, match := range propertyName.FindAllStringSubmatchIndex(path, -1) {
		full := path[match[0]:match[1]]
		key := full
		switch {
		case match[6] >= 0: // quoted
			key = escapeChar.ReplaceAllString(path[match[6]:match[7]], "$1")
		case match[2] >= 0: // bracket number or name
			key = path[match[2]:match[3]]
		case strings.HasPrefix(full, ".") || strings.HasPrefix(full, "[]"):
			key = ""
		}
		result = append(result, key)
	}
	return result
}

// Get ports lodash.get for JSON values; ok is false when undefined.
func Get(root any, path []string) (any, bool) {
	current := root
	for _, key := range path {
		switch v := current.(type) {
		case *Object:
			value, ok := v.Get(key)
			if !ok {
				return nil, false
			}
			if _, isUndefined := value.(undefined); isUndefined {
				return nil, false
			}
			current = value
		case []any:
			index, err := strconv.Atoi(key)
			if key == "length" {
				current = float64(len(v))
				continue
			}
			if err != nil || index < 0 || index >= len(v) {
				return nil, false
			}
			current = v[index]
			if _, isUndefined := current.(undefined); isUndefined {
				return nil, false
			}
		case string:
			if key == "length" {
				current = float64(len([]rune(v)))
				continue
			}
			return nil, false
		default:
			return nil, false
		}
	}
	return current, true
}

func isObjectLike(value any) bool {
	switch value.(type) {
	case *Object, []any:
		return true
	}
	return false
}

// Set ports lodash.set: missing or primitive intermediates are replaced by an
// array when the next key is an index, otherwise by an object. It returns the
// (possibly new) root.
func Set(root any, path []string, value any) any {
	if len(path) == 0 || !isObjectLike(root) {
		return root
	}
	setIn(root, path, value)
	return root
}

func setIn(container any, path []string, value any) any {
	key := path[0]
	last := len(path) == 1
	child := func(existing any, ok bool) any {
		if last {
			return value
		}
		if !ok || !isObjectLike(existing) {
			if indexKey.MatchString(path[1]) {
				existing = []any{}
			} else {
				existing = New()
			}
		}
		return setIn(existing, path[1:], value)
	}
	switch c := container.(type) {
	case *Object:
		existing, ok := c.Get(key)
		c.Set(key, child(existing, ok))
		return c
	case []any:
		index, err := strconv.Atoi(key)
		if err != nil || index < 0 {
			return c
		}
		for len(c) <= index {
			c = append(c, Undefined)
		}
		existing := c[index]
		_, isUndefined := existing.(undefined)
		c[index] = child(existing, !isUndefined)
		return c
	}
	return container
}

// Unset ports lodash.unset: deletes the last key; array slots become holes.
func Unset(root any, path []string) {
	if len(path) == 0 {
		return
	}
	parent, ok := Get(root, path[:len(path)-1])
	if !ok {
		return
	}
	key := path[len(path)-1]
	switch p := parent.(type) {
	case *Object:
		p.Delete(key)
	case []any:
		if index, err := strconv.Atoi(key); err == nil && index >= 0 && index < len(p) {
			p[index] = Undefined
		}
	}
}
