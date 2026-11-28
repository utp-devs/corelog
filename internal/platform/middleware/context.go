package middleware

import (
	"context"

	"github.com/google/uuid"
)

type contextKey string

const (
	userIDContextKey    contextKey = "auth.userID"
	roleContextKey      contextKey = "auth.role"
	sessionIDContextKey contextKey = "auth.sessionID"
)

func UserIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(userIDContextKey).(uuid.UUID)
	return id, ok
}

// SessionIDFromContext devuelve la sesion desde la que se hizo la peticion, si el middleware la validó.
func SessionIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(sessionIDContextKey).(uuid.UUID)
	return id, ok
}

func RoleFromContext(ctx context.Context) (string, bool) {
	role, ok := ctx.Value(roleContextKey).(string)
	return role, ok
}

const tokenIDContextKey contextKey = "auth.apiTokenID"

// APITokenIDFromContext returns the id of the API token that authenticated the request. It reports false for
// requests authenticated with a session.
func APITokenIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(tokenIDContextKey).(uuid.UUID)
	return id, ok
}
