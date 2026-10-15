// Package contracts owns the pinned Remnawave endpoint scope registry.
package contracts

import (
	_ "embed"
	"encoding/json"
	"strings"
)

//go:embed scopes.json
var Scopes []byte

type Endpoint struct {
	Key         string `json:"key"`
	Kind        string `json:"kind"`
	Method      string `json:"method"`
	Path        string `json:"path"`
	Description string `json:"description"`
	Resource    string `json:"-"`
}

var Endpoints = loadEndpoints()

func loadEndpoints() []Endpoint {
	var data struct {
		Resources []struct {
			Resource  string     `json:"resource"`
			Endpoints []Endpoint `json:"endpoints"`
		} `json:"resources"`
	}
	if err := json.Unmarshal(Scopes, &data); err != nil {
		panic(err)
	}
	var result []Endpoint
	for _, resource := range data.Resources {
		for _, endpoint := range resource.Endpoints {
			endpoint.Resource = resource.Resource
			result = append(result, endpoint)
		}
	}
	return result
}

func Lookup(method, path string) ([]Endpoint, bool) {
	segments := strings.Split(strings.Trim(path, "/"), "/")
	var best []Endpoint
	bestSpecificity := -1
	for _, endpoint := range Endpoints {
		if endpoint.Method != method {
			continue
		}
		pattern := strings.Split(strings.Trim(endpoint.Path, "/"), "/")
		if len(pattern) != len(segments) {
			continue
		}
		specificity, matches := 0, true
		for i, part := range pattern {
			if strings.HasPrefix(part, ":") || strings.HasPrefix(part, "{") {
				continue
			}
			if part != segments[i] {
				matches = false
				break
			}
			specificity++
		}
		if matches {
			if specificity > bestSpecificity {
				best, bestSpecificity = []Endpoint{endpoint}, specificity
			} else if specificity == bestSpecificity {
				best = append(best, endpoint)
			}
		}
	}
	return best, bestSpecificity >= 0
}

func AdminOnly(path string) bool {
	for _, prefix := range []string{"/api/tokens", "/api/auth", "/api/passkeys", "/api/remnawave-settings", "/api/node-ssh", "/api/system/updates", "/api/system/actions"} {
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return true
		}
	}
	return false
}

func Allows(scopes []string, endpoint Endpoint) bool {
	for _, scope := range scopes {
		if scope == "*" || scope == endpoint.Resource+":*" || scope == endpoint.Resource+":"+endpoint.Kind || scope == endpoint.Key {
			return true
		}
	}
	return false
}

// ValidScope reports whether scope exists in the catalog: "*", a resource
// wildcard, a resource read/write scope, or an endpoint key.
func ValidScope(scope string) bool {
	if scope == "*" {
		return true
	}
	for _, endpoint := range Endpoints {
		if scope == endpoint.Key || scope == endpoint.Resource+":*" || scope == endpoint.Resource+":read" || scope == endpoint.Resource+":write" {
			return true
		}
	}
	return false
}
