package port

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/user/domain"
)

// TokenStore persists single use tokens (password reset or email verification). Only the SHA-256 hash of a
// token is stored.
type TokenStore interface {
	// Replace discards the unused tokens of the user and stores the new one, so a user has at most one
	// active token.
	Replace(ctx context.Context, userID uuid.UUID, hash []byte, expiresAt, now time.Time) error
	// Find returns the owner of a token that is unused and unexpired at now, without consuming it.
	Find(ctx context.Context, hash []byte, now time.Time) (userID uuid.UUID, found bool, err error)
	// Consume marks the token as used when it exists, is unused and has not expired at now, and returns
	// its owner. It reports found=false otherwise. The operation is atomic: a token can be consumed once.
	Consume(ctx context.Context, hash []byte, now time.Time) (userID uuid.UUID, found bool, err error)
	// PurgeExpired deletes tokens that expired or were used before the instant.
	PurgeExpired(ctx context.Context, before time.Time) (int64, error)
}

// LockoutStore persists the failed login counters keyed by the hash of the normalized email.
type LockoutStore interface {
	Get(ctx context.Context, key string) (domain.LockoutState, error)
	Save(ctx context.Context, key string, state domain.LockoutState, now time.Time) error
	Delete(ctx context.Context, key string) error
	// PurgeStale deletes counters that are not locked and were last updated before the instant.
	PurgeStale(ctx context.Context, before time.Time) (int64, error)
}

// RecoveryService covers the password reset and email verification flows.
type RecoveryService interface {
	// RequestPasswordReset always succeeds from the caller's point of view: it never reveals whether the
	// email belongs to an account.
	RequestPasswordReset(ctx context.Context, email string) error
	// ResetPassword sets a new password from a reset token, revokes every session and clears the lockout.
	ResetPassword(ctx context.Context, in ResetPasswordInput) error
	// RequestEmailVerification sends a verification link to the address of the user.
	RequestEmailVerification(ctx context.Context, userID uuid.UUID) error
	// VerifyEmail marks the address as verified from a verification token.
	VerifyEmail(ctx context.Context, token string) error
}

// ResetPasswordInput is the payload of the reset.
type ResetPasswordInput struct {
	Token                string
	Password             string
	PasswordConfirmation string
}

// LoginGuard throttles password guessing per account and records the login audit events.
type LoginGuard interface {
	// Check returns locked=true with the remaining time when the account is locked.
	Check(ctx context.Context, email string) (locked bool, retryAfter time.Duration, err error)
	// RecordFailure counts a failed login; locked reports whether this failure started a lockout.
	RecordFailure(ctx context.Context, email string) (locked bool, retryAfter time.Duration, err error)
	// RecordSuccess clears the counter and records the login event.
	RecordSuccess(ctx context.Context, user *domain.User) error
}
