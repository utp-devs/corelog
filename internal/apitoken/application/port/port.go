// Package port declares what the API token use cases offer and need.
package port

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/apitoken/domain"
)

// Repository stores tokens.
type Repository interface {
	Insert(ctx context.Context, t *domain.Token, hash string) error
	// CountActive counts the tokens of a user that are neither revoked nor expired at now.
	CountActive(ctx context.Context, userID uuid.UUID, now time.Time) (int, error)
	// List returns the tokens of a user that are not logically deleted; includeRevoked also returns the revoked ones.
	List(ctx context.Context, userID uuid.UUID, includeRevoked bool) ([]domain.Token, error)
	// Revoke marks a token of the user as revoked and logically deleted. Missing or foreign tokens return domain.ErrNotFound.
	Revoke(ctx context.Context, userID, id uuid.UUID, at time.Time) (domain.Token, error)
	// FindByHash returns the token, its owner and the stored hash. A missing hash returns an error.
	FindByHash(ctx context.Context, hash string) (domain.Token, Owner, string, error)
	// Touch records the last use when it is missing or older than olderThan.
	Touch(ctx context.Context, id uuid.UUID, at time.Time, olderThan time.Time) error
}

// Owner is the user behind a token.
type Owner struct {
	Role        string
	Deactivated bool
}

// Identity is what a valid bearer token resolves to.
type Identity struct {
	UserID  uuid.UUID
	Role    string
	Scopes  []domain.Scope
	TokenID uuid.UUID
}

// Limiter decides whether a key may proceed. It matches platform/ratelimit.Limiter.
type Limiter interface {
	Allow(key string) (bool, time.Duration)
}

// CreateInput is a token creation request.
type CreateInput struct {
	Name       string
	Scopes     []string
	ExpiresAt  *time.Time
	GlobalRole string
}

// Service is the in-port.
type Service interface {
	// Create returns the stored token and the plaintext, which is never available again.
	Create(ctx context.Context, userID uuid.UUID, in CreateInput) (domain.Token, string, error)
	// List hides revoked tokens (logically deleted) unless includeRevoked is set.
	List(ctx context.Context, userID uuid.UUID, includeRevoked bool) ([]domain.Token, error)
	Revoke(ctx context.Context, userID, id uuid.UUID) error
	// Authenticate resolves a bearer plaintext to an identity: hash lookup, constant-time compare, expiry,
	// revocation, deactivated owners and per-token rate limit.
	Authenticate(ctx context.Context, plaintext string) (Identity, error)
}
