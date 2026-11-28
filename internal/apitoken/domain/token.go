// Package domain holds the personal API token rules: format, scopes, hashing and lifecycle checks.
package domain

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// Token format: "cl_pat_" followed by 40 random base62 characters.
const (
	TokenPrefix  = "cl_pat_"
	randomLength = 40
	// DisplayPrefixLength is how many leading characters are stored in clear to recognise a token.
	DisplayPrefixLength = len(TokenPrefix) + 4
	// MaxActivePerUser caps the live tokens of one user.
	MaxActivePerUser = 20
	// MaxLifetime is the longest expiry a token may be created with.
	MaxLifetime = 366 * 24 * time.Hour
	MaxNameLen  = 80
)

// Scope limits what a token may do.
type Scope string

const (
	ScopeRead  Scope = "read"
	ScopeWrite Scope = "write"
	ScopeAdmin Scope = "admin"
)

var (
	ErrNotFound = apperror.WithCode(apperror.KindNotFound, "api_token.not_found", "Token no encontrado")
	ErrInvalid  = apperror.WithCode(apperror.KindUnauthorized, "api_token.invalid", "Token no valido")
	// ErrSessionRequired is returned when a token tries to reach an endpoint that needs a real session.
	ErrSessionRequired = apperror.WithCode(apperror.KindForbidden, "api_token.session_required", "Esta accion requiere iniciar sesion; no se permite con un token de acceso")
	ErrScope           = apperror.WithCode(apperror.KindForbidden, "api_token.scope_insufficient", "El token no tiene permisos suficientes para esta operacion")
	ErrTooMany         = apperror.WithCode(apperror.KindConflict, "api_token.limit_reached", "Se alcanzo el maximo de 20 tokens activos; revoca alguno")
	ErrRateLimited     = apperror.WithCode(apperror.KindRateLimited, "api_token.rate_limited", "Demasiadas peticiones con este token; intenta de nuevo en unos instantes")
	ErrAdminScope      = apperror.WithCode(apperror.KindForbidden, "api_token.admin_scope_forbidden", "Solo los administradores de la plataforma pueden crear tokens con alcance admin")
)

func invalid(field, msg string) error {
	return apperror.WithCode(apperror.KindValidation, "api_token.invalid_input", "Token no valido: "+msg).
		WithDetails(apperror.FieldError{Field: field, Message: msg})
}

// Token is a stored personal access token. The plaintext is never part of it.
type Token struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	Name       string
	Prefix     string
	Scopes     []Scope
	ExpiresAt  *time.Time
	LastUsedAt *time.Time
	RevokedAt  *time.Time
	CreatedAt  time.Time
}

// Active reports whether the token can authenticate at the given instant.
func (t Token) Active(now time.Time) bool {
	if t.RevokedAt != nil {
		return false
	}
	return t.ExpiresAt == nil || now.Before(*t.ExpiresAt)
}

// Has reports whether the token carries the scope.
func (t Token) Has(s Scope) bool {
	for _, have := range t.Scopes {
		if have == s {
			return true
		}
	}
	return false
}

const alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

// Generate returns a new plaintext token using crypto/rand without modulo bias.
func Generate() (string, error) {
	out := make([]byte, 0, randomLength)
	buf := make([]byte, 64)
	for len(out) < randomLength {
		if _, err := rand.Read(buf); err != nil {
			return "", err
		}
		for _, b := range buf {
			if b >= 248 { // 248 = 62*4: rejecting the rest keeps the distribution uniform
				continue
			}
			out = append(out, alphabet[int(b)%len(alphabet)])
			if len(out) == randomLength {
				break
			}
		}
	}
	return TokenPrefix + string(out), nil
}

// Hash returns the hex SHA-256 of a plaintext token; only this value is stored.
func Hash(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}

// WellFormed reports whether the text has the shape of a token, before touching the database.
func WellFormed(s string) bool {
	if !strings.HasPrefix(s, TokenPrefix) || len(s) != len(TokenPrefix)+randomLength {
		return false
	}
	for _, c := range s[len(TokenPrefix):] {
		if !strings.ContainsRune(alphabet, c) {
			return false
		}
	}
	return true
}

// DisplayPrefix returns the part of the token that is stored and shown in lists.
func DisplayPrefix(plaintext string) string {
	if len(plaintext) < DisplayPrefixLength {
		return plaintext
	}
	return plaintext[:DisplayPrefixLength]
}

// ParseScopes validates, deduplicates and orders the requested scopes.
func ParseScopes(raw []string) ([]Scope, error) {
	if len(raw) == 0 {
		return nil, invalid("scopes", "indica al menos un alcance (read, write o admin)")
	}
	seen := map[Scope]bool{}
	for _, r := range raw {
		s := Scope(strings.TrimSpace(r))
		switch s {
		case ScopeRead, ScopeWrite, ScopeAdmin:
			seen[s] = true
		default:
			return nil, invalid("scopes", "alcance desconocido: "+r)
		}
	}
	out := make([]Scope, 0, len(seen))
	for s := range seen {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out, nil
}

// ValidateName trims and checks the display name.
func ValidateName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > MaxNameLen {
		return "", invalid("name", "el nombre es obligatorio y admite hasta 80 caracteres")
	}
	return name, nil
}

// ValidateExpiry checks an optional expiry against the clock and the maximum lifetime.
func ValidateExpiry(expires *time.Time, now time.Time) error {
	if expires == nil {
		return nil
	}
	if !expires.After(now) {
		return invalid("expires_at", "la fecha de vencimiento debe ser futura")
	}
	if expires.After(now.Add(MaxLifetime)) {
		return invalid("expires_at", "el vencimiento maximo es de un ano")
	}
	return nil
}

// AllowsMethod reports whether the scopes of a token allow an HTTP method: read tokens only get safe methods.
func AllowsMethod(scopes []Scope, method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return len(scopes) > 0
	}
	for _, s := range scopes {
		if s == ScopeWrite || s == ScopeAdmin {
			return true
		}
	}
	return false
}

// SessionOnlyPath reports whether a request path must never be served to a token: authentication,
// credentials, sessions, two-factor and token management itself.
func SessionOnlyPath(path string) bool {
	if i := strings.Index(path, "/api/v1"); i >= 0 {
		path = path[i+len("/api/v1"):]
	}
	path = strings.TrimSuffix(path, "/")
	for _, p := range []string{"/auth", "/me/api-tokens", "/me/password", "/me/sessions", "/me/deactivate", "/me/phone"} {
		if path == p || strings.HasPrefix(path, p+"/") {
			return true
		}
	}
	return false
}
