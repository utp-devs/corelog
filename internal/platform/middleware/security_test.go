package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jhonsferg/corelog/internal/platform/events"
)

func TestTrustedProxiesClientIP(t *testing.T) {
	t.Parallel()
	tp, bad := ParseTrustedProxies([]string{"10.0.0.0/8", "192.168.1.5", "garbage"})
	if len(bad) != 1 {
		t.Fatalf("bad = %v", bad)
	}
	tests := []struct {
		name, remote, xff, want string
	}{
		{"direct client ignores spoofed header", "203.0.113.7:5000", "1.2.3.4", "203.0.113.7"},
		{"proxy forwards client", "10.1.1.1:80", "198.51.100.2", "198.51.100.2"},
		{"chain of proxies takes the first untrusted from the right", "10.1.1.1:80", "6.6.6.6, 198.51.100.2, 10.2.2.2", "198.51.100.2"},
		{"single trusted address", "192.168.1.5:80", "198.51.100.9", "198.51.100.9"},
		{"proxy without header", "10.1.1.1:80", "", "10.1.1.1"},
		{"garbage header", "10.1.1.1:80", "nope", "10.1.1.1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.RemoteAddr = tt.remote
			if tt.xff != "" {
				r.Header.Set("X-Forwarded-For", tt.xff)
			}
			if got := tp.ClientIP(r); got != tt.want {
				t.Fatalf("ClientIP = %q, want %q", got, tt.want)
			}
		})
	}
	var none *TrustedProxies
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "10.1.1.1:80"
	r.Header.Set("X-Forwarded-For", "1.1.1.1")
	if got := none.ClientIP(r); got != "10.1.1.1" {
		t.Fatalf("nil proxies must trust nobody, got %q", got)
	}
}

func TestRequestMetaReachesTheContext(t *testing.T) {
	t.Parallel()
	var got events.RequestMeta
	h := RequestMeta(nil)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got, _ = events.RequestMetaFrom(r.Context())
	}))
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "198.51.100.4:1"
	r.Header.Set("User-Agent", "agent/1")
	h.ServeHTTP(httptest.NewRecorder(), r)
	if got.IP != "198.51.100.4" || got.UserAgent != "agent/1" {
		t.Fatalf("meta = %+v", got)
	}
}

func TestBodyLimitSkipsMultipartAndCapsStreams(t *testing.T) {
	t.Parallel()
	var readErr error
	h := BodyLimit(8)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 64)
		_, readErr = r.Body.Read(buf)
		w.WriteHeader(http.StatusNoContent)
	}))
	// Multipart bodies are left to the upload handlers.
	big := strings.Repeat("x", 100)
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(big))
	r.Header.Set("Content-Type", "multipart/form-data; boundary=x")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("multipart: %d", rec.Code)
	}
	// A body without a declared length is capped while reading.
	r = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(big))
	r.ContentLength = -1
	h.ServeHTTP(httptest.NewRecorder(), r)
	if readErr == nil {
		t.Fatal("an unbounded stream must hit the limit")
	}
}
