package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/authtoken"
)

const sessionTestSecret = "session-test-secret-long-enough-for-hs256"

// Estas pruebas registran un validador global: se limpia al terminar para no afectar a otras pruebas.
func TestAuthenticateRequiresActiveSessionWhenValidatorIsSet(t *testing.T) {
	userID := uuid.New()
	activeSession := uuid.New()
	revokedSession := uuid.New()
	SetSessionValidator(func(_ context.Context, sessionID, uid uuid.UUID) error {
		if sessionID == activeSession && uid == userID {
			return nil
		}
		return errors.New("revoked")
	})
	t.Cleanup(func() { SetSessionValidator(nil) })

	handler := Authenticate(sessionTestSecret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := SessionIDFromContext(r.Context())
		if !ok || id != activeSession {
			t.Errorf("session id not exposed in context: %v %v", id, ok)
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	cases := []struct {
		name  string
		token func() string
		want  int
	}{
		{"active session", func() string {
			tok, _ := authtoken.GenerateWithSession(sessionTestSecret, time.Hour, userID, "requester", activeSession)
			return tok
		}, http.StatusNoContent},
		{"revoked session", func() string {
			tok, _ := authtoken.GenerateWithSession(sessionTestSecret, time.Hour, userID, "requester", revokedSession)
			return tok
		}, http.StatusUnauthorized},
		{"token without session", func() string {
			tok, _ := authtoken.Generate(sessionTestSecret, time.Hour, userID, "requester")
			return tok
		}, http.StatusUnauthorized},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("Authorization", "Bearer "+tc.token())
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("status = %d; want %d", rec.Code, tc.want)
			}
		})
	}
}

func TestAuthenticateWithoutValidatorAcceptsAnySignedToken(t *testing.T) {
	SetSessionValidator(nil)
	handler := Authenticate(sessionTestSecret)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	tok, _ := authtoken.Generate(sessionTestSecret, time.Hour, uuid.New(), "requester")
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d; want 204 without a validator", rec.Code)
	}
}
