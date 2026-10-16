package prometheus

import (
	"context"
	"crypto/subtle"
	"log"
	"net"
	"net/http"
	"time"

	"gorm.io/gorm"

	"gowave/internal/metrics"
	"gowave/internal/subtpl/ojson"
)

// Handler serves what upstream's scheduler exposes on METRICS_PORT: /metrics
// behind basic auth and /health. WorkerRoutesGuard drops the connection for
// every other path.
func Handler(db *gorm.DB, user, pass string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/metrics":
			serveMetrics(w, r, user, pass)
		case r.Method == http.MethodGet && r.URL.Path == "/health":
			serveHealth(w, db)
		default:
			dropConnection(w)
		}
	})
}

func dropConnection(w http.ResponseWriter) {
	if hijacker, ok := w.(http.Hijacker); ok {
		if conn, _, err := hijacker.Hijack(); err == nil {
			_ = conn.Close()
			return
		}
	}
	panic(http.ErrAbortHandler)
}

func writeJSON(w http.ResponseWriter, status int, value *ojson.Object) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(ojson.Stringify(value)))
}

// serveMetrics ports PrometheusReporterController with BasicAuthGuard.
func serveMetrics(w http.ResponseWriter, r *http.Request, user, pass string) {
	w.Header().Set("WWW-Authenticate", `Basic realm="Prometheus Metrics"`)
	gotUser, gotPass, ok := r.BasicAuth()
	if !ok || user == "" || pass == "" ||
		subtle.ConstantTimeCompare([]byte(gotUser), []byte(user)) != 1 || subtle.ConstantTimeCompare([]byte(gotPass), []byte(pass)) != 1 {
		writeJSON(w, http.StatusUnauthorized, ojson.New("message", "Unauthorized", "statusCode", float64(401)))
		return
	}
	w.Header().Set("Content-Type", metrics.ContentType)
	_, _ = w.Write([]byte(metrics.Default.Text()))
}

// serveHealth ports HealthController (terminus with a database ping).
func serveHealth(w http.ResponseWriter, db *gorm.DB) {
	err := func() error {
		sqlDB, err := db.DB()
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return sqlDB.PingContext(ctx)
	}()
	if err != nil {
		down := ojson.New("database", ojson.New("status", "down", "message", err.Error()))
		writeJSON(w, http.StatusServiceUnavailable, ojson.New("status", "error", "info", ojson.New(), "error", down, "details", down))
		return
	}
	up := ojson.New("database", ojson.New("status", "up"))
	writeJSON(w, http.StatusOK, ojson.New("status", "ok", "info", up, "error", ojson.New(), "details", up))
}

// Serve listens on addr until ctx ends.
func Serve(ctx context.Context, addr string, handler http.Handler) {
	server := &http.Server{Addr: addr, Handler: handler, ReadHeaderTimeout: 15 * time.Second}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Printf("[METRICS] listen %s: %v", addr, err)
		return
	}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}()
	if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
		log.Printf("[METRICS] serve: %v", err)
	}
}
