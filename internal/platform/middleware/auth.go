package middleware

import (
	"context"
	"net/http"
	"strings"
	"sync/atomic"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/authtoken"
	"github.com/jhonsferg/corelog/internal/platform/httpresponse"
)

// SessionValidator confirma que la sesion sigue activa para el usuario: no revocada, no vencida
// y con la cuenta sin desactivar. Devuelve error si no lo está.
type SessionValidator func(ctx context.Context, sessionID, userID uuid.UUID) error

var sessionValidator atomic.Pointer[SessionValidator]

// SetSessionValidator registra el validador de sesiones para todo el proceso. Se llama una vez en el
// arranque (composition root). Sin validador, el middleware acepta cualquier JWT firmado (modo sin estado).
func SetSessionValidator(v SessionValidator) {
	if v == nil {
		sessionValidator.Store(nil)
		return
	}
	sessionValidator.Store(&v)
}

func Authenticate(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			token, ok := strings.CutPrefix(header, "Bearer ")
			if !ok || token == "" {
				httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "not authenticated"))
				return
			}

			claims, err := authtoken.Parse(secret, token)
			if err != nil {
				httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "not authenticated"))
				return
			}

			ctx := context.WithValue(r.Context(), userIDContextKey, claims.UserID)
			ctx = context.WithValue(ctx, roleContextKey, claims.Role)

			if v := sessionValidator.Load(); v != nil {
				sid := claims.SessionID()
				if sid == uuid.Nil || (*v)(ctx, sid, claims.UserID) != nil {
					httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "not authenticated"))
					return
				}
				ctx = context.WithValue(ctx, sessionIDContextKey, sid)
			}
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func RequireRole(allowed ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role, ok := RoleFromContext(r.Context())
			if !ok {
				httpresponse.Error(w, apperror.New(apperror.KindForbidden, "forbidden"))
				return
			}
			for _, a := range allowed {
				if role == a {
					next.ServeHTTP(w, r)
					return
				}
			}
			httpresponse.Error(w, apperror.New(apperror.KindForbidden, "forbidden"))
		})
	}
}
