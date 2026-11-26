package httpresponse

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

func TestHTTPResponse(t *testing.T) {
	t.Run("JSON writes data envelope", func(t *testing.T) {
		rec := httptest.NewRecorder()
		data := map[string]string{"message": "hello"}
		JSON(w(rec), http.StatusOK, data)

		if rec.Code != http.StatusOK {
			t.Errorf("got status %d, want %d", rec.Code, http.StatusOK)
		}
		if rec.Header().Get("Content-Type") != "application/json; charset=utf-8" {
			t.Errorf("got content-type %s", rec.Header().Get("Content-Type"))
		}

		var resp struct {
			Data map[string]string `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.Data["message"] != "hello" {
			t.Errorf("got message %q, want 'hello'", resp.Data["message"])
		}
	})

	t.Run("Error writes error envelope with mapped status code", func(t *testing.T) {
		tests := []struct {
			name       string
			err        error
			wantStatus int
		}{
			{"validation error -> 422", apperror.New(apperror.KindValidation, "invalid input"), http.StatusUnprocessableEntity},
			{"unauthorized error -> 401", apperror.New(apperror.KindUnauthorized, "not allowed"), http.StatusUnauthorized},
			{"forbidden error -> 403", apperror.New(apperror.KindForbidden, "access denied"), http.StatusForbidden},
			{"not found error -> 404", apperror.New(apperror.KindNotFound, "not found"), http.StatusNotFound},
			{"conflict error -> 409", apperror.New(apperror.KindConflict, "already exists"), http.StatusConflict},
			{"rate limited error -> 429", apperror.New(apperror.KindRateLimited, "too many requests"), http.StatusTooManyRequests},
			{"internal error -> 500", errors.New("raw db panic"), http.StatusInternalServerError},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				rec := httptest.NewRecorder()
				Error(rec, tt.err)

				if rec.Code != tt.wantStatus {
					t.Errorf("got status %d, want %d", rec.Code, tt.wantStatus)
				}

				var resp struct {
					Error struct {
						Message string `json:"message"`
					} `json:"error"`
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
					t.Fatalf("failed to parse error response: %v", err)
				}
				if resp.Error.Message == "" {
					t.Errorf("expected non-empty error message")
				}
			})
		}
	})
}

func w(rec *httptest.ResponseRecorder) http.ResponseWriter {
	return rec
}
