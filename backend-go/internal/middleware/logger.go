package middleware

import (
	"net/http"
	"strings"
	"time"

	"gowave/internal/httpx"
	"gowave/internal/logx"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

// Logger is the IS_HTTP_LOGGING_ENABLED access log, one line per request in
// the shape of upstream's morgan format. Query strings are left out: public
// subscription and API URLs can carry tokens.
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		wrapped := chimiddleware.NewWrapResponseWriter(w, r.ProtoMajor)

		next.ServeHTTP(wrapped, r)

		status := wrapped.Status()
		if status == 0 {
			status = http.StatusOK
		}
		agent := strings.ReplaceAll(r.UserAgent(), "\"", "'")
		logx.Infof("http", "%s %q %d %dB %s %q", httpx.ClientIP(r), r.Method+" "+r.URL.Path, status, wrapped.BytesWritten(), time.Since(start).Round(time.Microsecond), agent)
	})
}
