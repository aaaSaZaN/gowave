package middleware

import (
	"bytes"
	"errors"
	"io"
	"net/http"

	"gowave/internal/httpx"
)

// BodyLimit bounds the entire body before a handler can decode a JSON prefix or
// perform authentication work. Buffering is limited even without Content-Length.
func BodyLimit(limit int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.ContentLength > limit {
				httpx.Error(w, r, http.StatusRequestEntityTooLarge, "E000", "Request body too large")
				return
			}
			defer r.Body.Close()
			body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, limit))
			if err != nil {
				var sizeErr *http.MaxBytesError
				if errors.As(err, &sizeErr) {
					httpx.Error(w, r, http.StatusRequestEntityTooLarge, "E000", "Request body too large")
				} else {
					httpx.Error(w, r, http.StatusBadRequest, "E000", "Failed to read request body")
				}
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(body))
			next.ServeHTTP(w, r)
		})
	}
}
