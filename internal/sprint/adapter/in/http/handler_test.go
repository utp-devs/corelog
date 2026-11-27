package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// TestSprintHTTPRejectsMalformedDates comprueba que una fecha fuera de formato responde 422.
func TestSprintHTTPRejectsMalformedDates(t *testing.T) {
	bad := "2026/11/02"
	if _, err := parseDate(&bad); apperror.KindOf(err) != apperror.KindValidation {
		t.Fatalf("malformed date: want validation error, got %v", err)
	}
	good := "2026-11-02"
	start, err := parseDate(&good)
	if err != nil || start == nil {
		t.Fatalf("valid start date rejected: %v", err)
	}
}

// TestSprintHTTPRequiresAuthentication comprueba que sin identidad en el contexto la respuesta es 401.
func TestSprintHTTPRequiresAuthentication(t *testing.T) {
	h := NewHandler(nil, "secret")
	router := chi.NewRouter()
	router.Get("/sprints", h.List)

	request := httptest.NewRequest(http.MethodGet, "/sprints?project_id=00000000-0000-4000-8000-000000000001", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", recorder.Code)
	}
}
