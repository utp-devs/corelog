package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/authtoken"
	"github.com/jhonsferg/corelog/internal/platform/httpresponse"
)

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
