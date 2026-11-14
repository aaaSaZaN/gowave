package middleware_test

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gowave/internal/middleware"
)

type trackedBody struct {
	io.Reader
	read   int
	closed bool
}

func (b *trackedBody) Read(p []byte) (int, error) {
	n, err := b.Reader.Read(p)
	b.read += n
	return n, err
}

func (b *trackedBody) Close() error {
	b.closed = true
	return nil
}

func TestBodyLimit(t *testing.T) {
	const limit = 16 << 10
	for _, size := range []int{0, limit - 1, limit, limit + 1, 1 << 20} {
		for _, unknownLength := range []bool{false, true} {
			payload := strings.Repeat("x", size)
			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(payload))
			if unknownLength {
				req.ContentLength = -1
			}
			body := &trackedBody{Reader: req.Body}
			req.Body = body
			called := false
			handler := middleware.BodyLimit(limit)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				got, err := io.ReadAll(r.Body)
				if err != nil || string(got) != payload {
					t.Fatalf("handler body differs: size=%d, error=%v", len(got), err)
				}
				w.WriteHeader(http.StatusNoContent)
			}))
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			want := http.StatusNoContent
			if size > limit {
				want = http.StatusRequestEntityTooLarge
			}
			if w.Code != want || called != (size <= limit) {
				t.Fatalf("size=%d unknownLength=%t: status=%d called=%t, want status=%d", size, unknownLength, w.Code, called, want)
			}
			maxRead := size
			if size > limit {
				maxRead = 0
				if unknownLength {
					maxRead = limit + 1
				}
			}
			if body.read > maxRead {
				t.Fatalf("read %d bytes, maximum %d", body.read, maxRead)
			}
			if (size <= limit || unknownLength) && !body.closed {
				t.Fatal("buffered request body was not closed")
			}
		}
	}
}

type failedBody struct{}

func (failedBody) Read([]byte) (int, error) { return 0, errors.New("read failed") }
func (failedBody) Close() error             { return nil }

func TestBodyLimitReadFailure(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Body = failedBody{}
	w := httptest.NewRecorder()
	middleware.BodyLimit(16<<10)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("handler called after a body read failure")
	})).ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400", w.Code)
	}
}
