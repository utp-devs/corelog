package middleware

import (
	"context"
	"net/http"
	"strings"
	"sync/atomic"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/httpresponse"
)

// APITokenPrefix is the start of every personal API token.
const APITokenPrefix = "cl_pat_"

// APITokenIdentity is what a valid API token resolves to. Role is the platform role the request runs with (the
// token only keeps "admin" when it carries the admin scope).
type APITokenIdentity struct {
	UserID  uuid.UUID
	Role    string
	TokenID uuid.UUID
	// AllowsMethod tells whether the token scopes allow the HTTP method.
	AllowsMethod func(method string) bool
}

// APITokenAuthenticator resolves a bearer token. It returns an apperror (401 for invalid tokens, 429 when limited).
type APITokenAuthenticator func(ctx context.Context, plaintext string) (APITokenIdentity, error)

// APITokenSessionOnly reports whether a path must never be served to an API token.
type APITokenSessionOnly func(path string) bool

type tokenAuth struct {
	authenticate APITokenAuthenticator
	sessionOnly  APITokenSessionOnly
}

var apiTokens atomic.Pointer[tokenAuth]

// SetAPITokenAuthenticator registers the bearer token authentication for the whole process. It is called once in
// the composition root. Without it, bearer values that look like API tokens are rejected.
func SetAPITokenAuthenticator(a APITokenAuthenticator, sessionOnly APITokenSessionOnly) {
	if a == nil {
		apiTokens.Store(nil)
		return
	}
	apiTokens.Store(&tokenAuth{authenticate: a, sessionOnly: sessionOnly})
}

// authenticateToken handles "Authorization: Bearer cl_pat_..." and reports whether the request was served.
func authenticateToken(w http.ResponseWriter, r *http.Request, token string, next http.Handler) {
	ta := apiTokens.Load()
	if ta == nil {
		httpresponse.Error(w, apperror.New(apperror.KindUnauthorized, "not authenticated"))
		return
	}
	if ta.sessionOnly != nil && ta.sessionOnly(r.URL.Path) {
		httpresponse.Error(w, apperror.WithCode(apperror.KindForbidden, "api_token.session_required",
			"Esta accion requiere iniciar sesion; no se permite con un token de acceso"))
		return
	}
	id, err := ta.authenticate(r.Context(), token)
	if err != nil {
		httpresponse.Error(w, err)
		return
	}
	if id.AllowsMethod != nil && !id.AllowsMethod(r.Method) {
		httpresponse.Error(w, apperror.WithCode(apperror.KindForbidden, "api_token.scope_insufficient",
			"El token no tiene permisos suficientes para esta operacion"))
		return
	}
	ctx := context.WithValue(r.Context(), userIDContextKey, id.UserID)
	ctx = context.WithValue(ctx, roleContextKey, id.Role)
	ctx = context.WithValue(ctx, tokenIDContextKey, id.TokenID)
	next.ServeHTTP(w, r.WithContext(ctx))
}

func isAPIToken(token string) bool { return strings.HasPrefix(token, APITokenPrefix) }
