package middleware

import (
	"net/http"

	"gowave/internal/contracts"
)

// statusWriter rewrites any 2xx a handler writes to the success status the
// upstream controller returns for that command, and drops the body for 204.
type statusWriter struct {
	http.ResponseWriter
	want        int
	wroteHeader bool
	noBody      bool
}

func (w *statusWriter) WriteHeader(code int) {
	if w.wroteHeader {
		return
	}
	w.wroteHeader = true
	if code >= 200 && code < 300 {
		code = w.want
	}
	if code == http.StatusNoContent {
		w.noBody = true
		w.Header().Del("Content-Type")
		w.Header().Del("Content-Length")
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(data []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	if w.noBody {
		return len(data), nil
	}
	return w.ResponseWriter.Write(data)
}

func (w *statusWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// Unwrap exposes the underlying writer to http.ResponseController.
func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// ContractStatus aligns success codes with the pinned contract. Errors and
// non-contract routes pass through untouched.
func ContractStatus(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		command, ok := contracts.FindCommand(r.Method, r.URL.Path)
		if !ok {
			next.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(&statusWriter{ResponseWriter: w, want: command.Status}, r)
	})
}
