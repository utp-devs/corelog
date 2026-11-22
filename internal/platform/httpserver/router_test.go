package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jhonsferg/corelog/internal/platform/config"
)

func TestNewRouter(t *testing.T) {
	cfg := config.ServerConfig{
		AllowedOrigins: []string{"http://localhost:5173"},
	}

	router := NewRouter(cfg)

	t.Run("GET /health returns status 200 and json ok", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("got status %d, want %d", rec.Code, http.StatusOK)
		}
		if rec.Header().Get("Content-Type") != "application/json; charset=utf-8" {
			t.Errorf("got content type %s", rec.Header().Get("Content-Type"))
		}
		expected := `{"status":"ok"}`
		if rec.Body.String() != expected {
			t.Errorf("got body %q, want %q", rec.Body.String(), expected)
		}
	})
}
