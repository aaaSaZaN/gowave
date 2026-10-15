package system

import (
	"github.com/go-chi/chi/v5"
	"net/http"
	"sort"
	"strings"
	"sync"
)

type RouteCounter struct {
	mu     sync.RWMutex
	counts map[string]int
	total  int
}

var GlobalRouteCounter = &RouteCounter{
	counts: make(map[string]int),
}

func (rc *RouteCounter) Inc(method, route string) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	key := method + " " + route
	if _, exists := rc.counts[key]; !exists && len(rc.counts) >= 2048 {
		key = "OTHER unmatched"
	}
	rc.counts[key]++
	rc.total++
}

func (rc *RouteCounter) GetStats() (int, []map[string]interface{}) {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	routes := make([]map[string]interface{}, 0, len(rc.counts))
	for k, count := range rc.counts {
		parts := strings.SplitN(k, " ", 2)
		if len(parts) == 2 {
			routes = append(routes, map[string]interface{}{
				"method": parts[0],
				"route":  parts[1],
				"count":  count,
			})
		}
	}
	sort.Slice(routes, func(i, j int) bool {
		return routes[i]["count"].(int) > routes[j]["count"].(int)
	})
	return rc.total, routes
}

func RouteCounterMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
		path := ""
		if ctx := chi.RouteContext(r.Context()); ctx != nil {
			path = ctx.RoutePattern()
		}
		if path == "" {
			path = "unmatched"
		}
		if !strings.HasPrefix(r.URL.Path, "/assets") &&
			!strings.HasPrefix(r.URL.Path, "/favicons") &&
			!strings.HasPrefix(r.URL.Path, "/locales") &&
			!strings.HasPrefix(r.URL.Path, "/lotties") &&
			!strings.HasPrefix(r.URL.Path, "/splash_screens") {
			GlobalRouteCounter.Inc(r.Method, path)
		}
	})
}
