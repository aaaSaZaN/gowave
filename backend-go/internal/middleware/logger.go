package middleware

import (
	"fmt"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"net/http"
	"time"
)

func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		wrapped := chimiddleware.NewWrapResponseWriter(w, r.ProtoMajor)

		next.ServeHTTP(wrapped, r)

		duration := time.Since(start)
		status := wrapped.Status()
		if status == 0 {
			status = http.StatusOK
		}
		fmt.Printf("[%s] %s %d (%v)\n", r.Method, r.URL.Path, status, duration)
	})
}
