package subtpl

import (
	"strings"

	"gowave/internal/subtpl/ojson"
)

const hostSourcePrefix = "$host."

var hostAllowedPaths = func() [][]string {
	var paths [][]string
	for _, p := range []string{"address", "clientOverrides.serverDescription", "finalRemark", "metadata.inboundTag", "metadata.remark", "metadata.tags", "mux", "port", "protocol", "protocolOptions", "security", "securityOptions", "streamOverrides.finalMask", "streamOverrides.sockopt", "transport", "transportOptions"} {
		paths = append(paths, ojson.ToPath(p))
	}
	return paths
}()

func isAllowedHostPath(segments []string) bool {
	for _, allowed := range hostAllowedPaths {
		if len(allowed) > len(segments) {
			continue
		}
		match := true
		for i, segment := range allowed {
			if segments[i] != segment {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func isBlockedByPrimitive(target any, path string) bool {
	segments := ojson.ToPath(path)
	for index := 1; index < len(segments); index++ {
		value, ok := ojson.Get(target, segments[:index])
		if !ok || value == nil {
			continue
		}
		switch value.(type) {
		case *ojson.Object, []any:
		default:
			return true
		}
	}
	return false
}

// applyHostMapper ports applyHostMapper: copy/set/unset operations on a
// clone of target. flatTargets treats "to" as a single key.
func applyHostMapper(target any, operations []any, host *Resolved, flatTargets bool) any {
	if len(operations) == 0 {
		return target
	}
	result := ojson.Clone(target)
	var hostObject *ojson.Object
	for _, raw := range operations {
		op, ok := raw.(*ojson.Object)
		if !ok {
			continue
		}
		to, _ := op.Value("to").(string)
		path := ojson.ToPath(to)
		if flatTargets {
			path = []string{to}
		} else if isBlockedByPrimitive(result, to) {
			continue
		}
		switch op.Value("op") {
		case "copy":
			from, _ := op.Value("from").(string)
			var root any
			var source []string
			if !strings.HasPrefix(from, hostSourcePrefix) {
				rawInbound, ok := host.Meta.RawInbound.(*ojson.Object)
				if !ok || rawInbound == nil {
					continue
				}
				root, source = rawInbound, ojson.ToPath(from)
			} else {
				segments := ojson.ToPath(from[len(hostSourcePrefix):])
				if len(segments) == 0 || !isAllowedHostPath(segments) {
					continue
				}
				if hostObject == nil {
					hostObject = host.Object()
				}
				root, source = hostObject, segments
			}
			value, ok := ojson.Get(root, source)
			if !ok {
				continue
			}
			result = ojson.Set(result, path, ojson.Clone(value))
		case "set":
			value, ok := op.Get("value")
			if !ok {
				value = ojson.Undefined
			}
			result = ojson.Set(result, path, ojson.Clone(value))
		case "unset":
			ojson.Unset(result, path)
		}
	}
	return result
}
