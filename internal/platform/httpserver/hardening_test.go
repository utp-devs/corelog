package httpserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jhonsferg/corelog/internal/platform/config"
)

func TestSecurityHeadersOnResponses(t *testing.T) {
	t.Parallel()
	h := NewRouterWith(config.ServerConfig{AllowedOrigins: []string{"http://app.test"}}, RouterOptions{HSTS: true})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
	for k, want := range map[string]string{
		"X-Content-Type-Options":    "nosniff",
		"X-Frame-Options":           "DENY",
		"Referrer-Policy":           "no-referrer",
		"Content-Security-Policy":   "default-src 'none'; frame-ancestors 'none'",
		"Strict-Transport-Security": "max-age=63072000; includeSubDomains",
	} {
		if got := rec.Header().Get(k); got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}
	if rec.Header().Get("Permissions-Policy") == "" {
		t.Error("Permissions-Policy missing")
	}
	// Without HSTS enabled the header is absent.
	rec = httptest.NewRecorder()
	NewRouter(config.ServerConfig{}).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
	if rec.Header().Get("Strict-Transport-Security") != "" {
		t.Error("HSTS must be off by default")
	}
}

func TestCORSMatrix(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		cfg     config.ServerConfig
		origin  string
		wantACO string
	}{
		{"allowed", config.ServerConfig{CorsAllowedOrigins: []string{"http://app.test"}}, "http://app.test", "http://app.test"},
		{"other origin denied", config.ServerConfig{CorsAllowedOrigins: []string{"http://app.test"}}, "http://evil.test", ""},
		{"wildcard is dropped", config.ServerConfig{CorsAllowedOrigins: []string{"*"}}, "http://evil.test", ""},
		{"legacy variable", config.ServerConfig{AllowedOrigins: []string{"http://old.test"}}, "http://old.test", "http://old.test"},
		{"plural wins", config.ServerConfig{AllowedOrigins: []string{"http://old.test"}, CorsAllowedOrigins: []string{"http://new.test"}}, "http://old.test", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(http.MethodGet, "/health", nil)
			req.Header.Set("Origin", tt.origin)
			rec := httptest.NewRecorder()
			NewRouter(tt.cfg).ServeHTTP(rec, req)
			if got := rec.Header().Get("Access-Control-Allow-Origin"); got != tt.wantACO {
				t.Fatalf("Allow-Origin = %q, want %q", got, tt.wantACO)
			}
			if tt.wantACO != "" && rec.Header().Get("Access-Control-Allow-Credentials") != "true" {
				t.Fatal("credentials header missing for an allowed origin")
			}
		})
	}
}

func TestBodyLimitAnswers413(t *testing.T) {
	t.Parallel()
	r := NewRouterWith(config.ServerConfig{}, RouterOptions{MaxJSONBodyBytes: 16})
	r.Post("/echo", func(w http.ResponseWriter, req *http.Request) { w.WriteHeader(http.StatusNoContent) })
	req := httptest.NewRequest(http.MethodPost, "/echo", strings.NewReader(strings.Repeat("a", 64)))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusRequestEntityTooLarge || !strings.Contains(rec.Body.String(), "request.too_large") {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body)
	}
	small := httptest.NewRequest(http.MethodPost, "/echo", strings.NewReader("ok"))
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, small)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("small body code = %d", rec.Code)
	}
}
