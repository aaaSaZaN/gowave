// Package httpx writes responses in the envelope and error shapes used by the
// upstream Remnawave backend.
package httpx

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"time"
)

// Error codes from @remnawave/backend-contract constants/errors.
const (
	CodeInternal      = "A001"
	CodeUnsupported   = "GW501"
	CodeInvalidRange  = "A235"
	CodeSquadNotFound = "A118"
)

// JSON writes value with the given status.
func JSON(w http.ResponseWriter, status int, value interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

// Response writes {"response": value} with status 200.
func Response(w http.ResponseWriter, value interface{}) {
	JSON(w, http.StatusOK, map[string]interface{}{"response": value})
}

// Error writes the upstream HttpExceptionFilter body.
func Error(w http.ResponseWriter, r *http.Request, status int, code, message string) {
	if status == http.StatusForbidden {
		message = "Forbidden"
	}
	JSON(w, status, map[string]interface{}{
		"timestamp": time.Now().UTC().Format("2006-01-02T15:04:05.000Z"),
		"path":      r.URL.RequestURI(),
		"message":   message,
		"errorCode": code,
	})
}

// Unsupported reports an operation this deployment cannot perform, for example
// a capability a connected node agent lacks. Clients must not treat it as
// success.
func Unsupported(w http.ResponseWriter, r *http.Request, message string) {
	Error(w, r, http.StatusNotImplemented, CodeUnsupported, message)
}

// Decode reads a JSON body of at most limit bytes into target.
func Decode(r *http.Request, limit int64, target interface{}) error {
	return json.NewDecoder(http.MaxBytesReader(nil, r.Body, limit)).Decode(target)
}

// ClientIP follows the request-ip package used upstream: well-known proxy
// headers first, then the socket address. Only meaningful behind a proxy that
// overwrites these headers.
//
// X-Remnawave-Real-IP comes first: the subscription page sends the client's
// address there and, for an http:// panel URL, sets X-Forwarded-For to
// 127.0.0.1 on every request.
func ClientIP(r *http.Request) string {
	for _, header := range []string{"X-Remnawave-Real-IP", "X-Client-IP", "X-Forwarded-For", "CF-Connecting-IP", "Fastly-Client-Ip", "True-Client-Ip", "X-Real-IP", "X-Cluster-Client-IP", "X-Forwarded", "Forwarded-For"} {
		if value := r.Header.Get(header); value != "" {
			first := strings.TrimSpace(strings.Split(value, ",")[0])
			if host, _, err := net.SplitHostPort(first); err == nil {
				first = host
			}
			if net.ParseIP(first) != nil {
				return first
			}
		}
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}
