package port

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/user/domain"
)

// SecretCipher encrypts values stored at rest. The additional data binds a sealed value to its owner.
type SecretCipher interface {
	Seal(plaintext, additional []byte) ([]byte, error)
	Open(sealed, additional []byte) ([]byte, error)
}

// TwoFactorSetup is the stored TOTP configuration of one user. It is usable for login only when EnabledAt is set.
type TwoFactorSetup struct {
	UserID          uuid.UUID
	SecretEncrypted []byte
	EnabledAt       *time.Time
	LastUsedStep    *int64
}

// RecoveryCodeRecord is one stored recovery code. Only the SHA-256 digest is kept.
type RecoveryCodeRecord struct {
	ID       uuid.UUID
	CodeHash []byte
}

// LoginChallenge is the server side state of a login that waits for the second factor.
type LoginChallenge struct {
	ID             uuid.UUID
	UserID         uuid.UUID
	ExpiresAt      time.Time
	FailedAttempts int
	ConsumedAt     *time.Time
}

// TwoFactorRepository persists TOTP setups, recovery codes and login challenges. Every write is scoped to
// one user.
type TwoFactorRepository interface {
	// FindSetup returns the setup of the user, or nil when the user never started enrollment.
	FindSetup(ctx context.Context, userID uuid.UUID) (*TwoFactorSetup, error)
	// SaveSetup stores a pending (not yet enabled) secret, replacing any previous pending secret. It returns
	// false, and stores nothing, when the user already has an enabled setup.
	SaveSetup(ctx context.Context, userID uuid.UUID, secretEncrypted []byte) (bool, error)
	// MarkEnabled sets enabled_at and the step that confirmed the enrollment. It returns false when the setup
	// was already enabled.
	MarkEnabled(ctx context.Context, userID uuid.UUID, at time.Time, step int64) (bool, error)
	// AdvanceLastUsedStep stores step only when it is greater than the stored one. It returns false when the
	// step was already used, which makes replayed codes fail even when two requests race.
	AdvanceLastUsedStep(ctx context.Context, userID uuid.UUID, step int64) (bool, error)
	ListUnusedRecoveryCodes(ctx context.Context, userID uuid.UUID) ([]RecoveryCodeRecord, error)
	CountUnusedRecoveryCodes(ctx context.Context, userID uuid.UUID) (int, error)
	// MarkRecoveryCodeUsed consumes one code. It returns false when the code was already used.
	MarkRecoveryCodeUsed(ctx context.Context, userID, codeID uuid.UUID, at time.Time) (bool, error)
	// ReplaceRecoveryCodes deletes the codes of the user and stores the given digests.
	ReplaceRecoveryCodes(ctx context.Context, userID uuid.UUID, hashes [][]byte) error
	// Delete removes the setup and every recovery code of the user.
	Delete(ctx context.Context, userID uuid.UUID) error

	CreateChallenge(ctx context.Context, userID uuid.UUID, expiresAt time.Time) (uuid.UUID, error)
	// FindChallenge returns the challenge, or nil when it does not exist.
	FindChallenge(ctx context.Context, challengeID uuid.UUID) (*LoginChallenge, error)
	// RecordFailedChallengeAttempt counts a wrong code. The challenge is consumed when the count reaches maxAttempts.
	RecordFailedChallengeAttempt(ctx context.Context, challengeID uuid.UUID, maxAttempts int, at time.Time) error
	// ConsumeChallenge marks the challenge used. It returns false when the challenge is already consumed,
	// expired or exhausted.
	ConsumeChallenge(ctx context.Context, challengeID uuid.UUID, at time.Time, maxAttempts int) (bool, error)
}

// TwoFactorStatus describes the second factor of the user.
type TwoFactorStatus struct {
	Enabled                bool
	EnabledAt              *time.Time
	RecoveryCodesRemaining int
}

// TwoFactorEnrollment carries the secret once, when enrollment starts.
type TwoFactorEnrollment struct {
	Secret     string
	OTPAuthURI string
}

// CompleteLoginInput carries either a TOTP code or a recovery code.
type CompleteLoginInput struct {
	Code         string
	RecoveryCode string
}

// TwoFactorService is the in-port of the second factor.
type TwoFactorService interface {
	Status(ctx context.Context, userID uuid.UUID) (TwoFactorStatus, error)
	// IsEnabled reports whether a login of the user must continue with a second factor.
	IsEnabled(ctx context.Context, userID uuid.UUID) (bool, error)
	// Enroll stores a new pending secret and returns it once.
	Enroll(ctx context.Context, userID uuid.UUID) (TwoFactorEnrollment, error)
	// ConfirmEnrollment checks a code, enables the second factor and returns the recovery codes once.
	ConfirmEnrollment(ctx context.Context, userID uuid.UUID, code string) ([]string, error)
	// Disable turns the second factor off. It needs the current password and a TOTP or recovery code.
	Disable(ctx context.Context, userID uuid.UUID, password string, in CompleteLoginInput) error
	// RegenerateRecoveryCodes replaces the recovery codes after a valid TOTP code.
	RegenerateRecoveryCodes(ctx context.Context, userID uuid.UUID, code string) ([]string, error)
	// StartChallenge opens a login challenge for a user with the second factor enabled.
	StartChallenge(ctx context.Context, userID uuid.UUID) (challengeID uuid.UUID, expiresAt time.Time, err error)
	// CompleteChallenge verifies the second factor of a challenge and returns the user who logs in.
	CompleteChallenge(ctx context.Context, challengeID, userID uuid.UUID, in CompleteLoginInput) (*domain.User, error)
}
