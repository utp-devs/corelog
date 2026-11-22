package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/authtoken"
)

func TestAuthMiddlewares(t *testing.T) {
	secret := "test-secret-at-least-32-chars-long"
	adminID := uuid.New()
	requesterID := uuid.New()

	adminToken, _ := authtoken.Generate(secret, time.Hour, adminID, "admin")
	requesterToken, _ := authtoken.Generate(secret, time.Hour, requesterID, "requester")

	dummyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uid, ok := UserIDFromContext(r.Context())
		if !ok {
			http.Error(w, "no user in context", http.StatusInternalServerError)
			return
		}
		role, _ := RoleFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(uid.String() + ":" + role))
	})

	t.Run("Authenticate passes with valid token and sets context", func(t *testing.T) {
		h := Authenticate(secret)(dummyHandler)
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("got status %d, want %d", rec.Code, http.StatusOK)
		}
		expectedBody := adminID.String() + ":admin"
		if rec.Body.String() != expectedBody {
			t.Errorf("got body %q, want %q", rec.Body.String(), expectedBody)
		}
	})

	t.Run("Authenticate rejects missing or invalid header", func(t *testing.T) {
		h := Authenticate(secret)(dummyHandler)

		req1 := httptest.NewRequest(http.MethodGet, "/", nil)
		rec1 := httptest.NewRecorder()
		h.ServeHTTP(rec1, req1)
		if rec1.Code != http.StatusUnauthorized {
			t.Errorf("missing header: got status %d, want %d", rec1.Code, http.StatusUnauthorized)
		}
		if rec1.Header().Get("Content-Type") != "application/json; charset=utf-8" || rec1.Body.String() != "{\"error\":{\"message\":\"not authenticated\"}}\n" {
			t.Errorf("missing header response is not a structured API error: headers=%v body=%q", rec1.Header(), rec1.Body.String())
		}

		req2 := httptest.NewRequest(http.MethodGet, "/", nil)
		req2.Header.Set("Authorization", "Bearer invalid-token")
		rec2 := httptest.NewRecorder()
		h.ServeHTTP(rec2, req2)
		if rec2.Code != http.StatusUnauthorized {
			t.Errorf("invalid token: got status %d, want %d", rec2.Code, http.StatusUnauthorized)
		}
		if rec2.Body.String() != "{\"error\":{\"message\":\"not authenticated\"}}\n" {
			t.Errorf("invalid token response is not a structured API error: body=%q", rec2.Body.String())
		}
	})

	t.Run("RequireRole allows matching role", func(t *testing.T) {
		h := Authenticate(secret)(RequireRole("admin")(dummyHandler))
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("got status %d, want %d", rec.Code, http.StatusOK)
		}
	})

	t.Run("RequireRole rejects non-matching role", func(t *testing.T) {
		h := Authenticate(secret)(RequireRole("admin")(dummyHandler))
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer "+requesterToken)
		rec := httptest.NewRecorder()

		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Errorf("got status %d, want %d", rec.Code, http.StatusForbidden)
		}
		if rec.Header().Get("Content-Type") != "application/json; charset=utf-8" || rec.Body.String() != "{\"error\":{\"message\":\"forbidden\"}}\n" {
			t.Errorf("role rejection response is not a structured API error: headers=%v body=%q", rec.Header(), rec.Body.String())
		}
	})
}
