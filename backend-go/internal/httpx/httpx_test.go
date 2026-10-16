package httpx

import (
	"net/http/httptest"
	"testing"
)

func TestClientIP(t *testing.T) {
	cases := []struct {
		name    string
		headers map[string]string
		want    string
	}{
		{
			// Headers the subscription page sends to an http:// panel URL.
			name:    "subscription page real ip",
			headers: map[string]string{"X-Remnawave-Real-IP": "203.0.113.7", "X-Forwarded-For": "127.0.0.1"},
			want:    "203.0.113.7",
		},
		{
			name:    "invalid real ip header falls through",
			headers: map[string]string{"X-Remnawave-Real-IP": "unknown", "X-Forwarded-For": "198.51.100.4, 10.0.0.1"},
			want:    "198.51.100.4",
		},
		{
			name:    "reverse proxy",
			headers: map[string]string{"X-Real-IP": "2001:db8::1"},
			want:    "2001:db8::1",
		},
		{
			name: "socket address",
			want: "192.0.2.1",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/api/sub/abc", nil)
			for key, value := range tc.headers {
				r.Header.Set(key, value)
			}
			if got := ClientIP(r); got != tc.want {
				t.Fatalf("ClientIP() = %q, want %q", got, tc.want)
			}
		})
	}
}
