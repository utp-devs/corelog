package usecase

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/platform/database"
	"github.com/jhonsferg/corelog/internal/platform/events"
	"github.com/jhonsferg/corelog/internal/user/application/port"
	"github.com/jhonsferg/corelog/internal/user/domain"
)

// twoFactorIssuer is the name shown by authenticator apps next to the account.
const twoFactorIssuer = "CoreLog"

// Event types written for security sensitive changes of the second factor.
const (
	EventTwoFactorEnabled         = "user.two_factor_enabled"
	EventTwoFactorDisabled        = "user.two_factor_disabled"
	EventRecoveryCodeUsed         = "user.recovery_code_used"
	EventRecoveryCodesRegenerated = "user.recovery_codes_regenerated"
)

// TwoFactorService implements TOTP enrollment, the login challenge and recovery codes.
type TwoFactorService struct {
	users     port.Repository
	repo      port.TwoFactorRepository
	cipher    port.SecretCipher
	tx        database.TxManager
	publisher events.Publisher
	clock     clock.Clock
}

// NewTwoFactorService wires the second factor use cases. Every write that must be atomic runs inside tx,
// and the audit events are published through publisher inside the same transaction.
func NewTwoFactorService(users port.Repository, repo port.TwoFactorRepository, cipher port.SecretCipher,
	tx database.TxManager, publisher events.Publisher, clk clock.Clock) *TwoFactorService {
	return &TwoFactorService{users: users, repo: repo, cipher: cipher, tx: tx, publisher: publisher, clock: clk}
}

var _ port.TwoFactorService = (*TwoFactorService)(nil)

// secretAAD binds an encrypted secret to its user so that a row cannot be moved to another account.
func secretAAD(userID uuid.UUID) []byte {
	return []byte("two_factor_secret:" + userID.String())
}

func (s *TwoFactorService) Status(ctx context.Context, userID uuid.UUID) (port.TwoFactorStatus, error) {
	setup, err := s.repo.FindSetup(ctx, userID)
	if err != nil {
		return port.TwoFactorStatus{}, err
	}
	if setup == nil || setup.EnabledAt == nil {
		return port.TwoFactorStatus{}, nil
	}
	remaining, err := s.repo.CountUnusedRecoveryCodes(ctx, userID)
	if err != nil {
		return port.TwoFactorStatus{}, err
	}
	return port.TwoFactorStatus{Enabled: true, EnabledAt: setup.EnabledAt, RecoveryCodesRemaining: remaining}, nil
}

func (s *TwoFactorService) IsEnabled(ctx context.Context, userID uuid.UUID) (bool, error) {
	setup, err := s.repo.FindSetup(ctx, userID)
	if err != nil {
		return false, err
	}
	return setup != nil && setup.EnabledAt != nil, nil
}

func (s *TwoFactorService) Enroll(ctx context.Context, userID uuid.UUID) (port.TwoFactorEnrollment, error) {
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return port.TwoFactorEnrollment{}, err
	}
	secret, err := domain.NewTOTPSecret()
	if err != nil {
		return port.TwoFactorEnrollment{}, apperror.Wrap(apperror.KindInternal, "failed to create two-factor secret", err)
	}
	sealed, err := s.cipher.Seal(secret, secretAAD(userID))
	if err != nil {
		return port.TwoFactorEnrollment{}, apperror.Wrap(apperror.KindInternal, "failed to protect two-factor secret", err)
	}
	saved, err := s.repo.SaveSetup(ctx, userID, sealed)
	if err != nil {
		return port.TwoFactorEnrollment{}, err
	}
	if !saved {
		return port.TwoFactorEnrollment{}, domain.ErrTwoFactorAlreadyEnabled
	}
	return port.TwoFactorEnrollment{
		Secret:     secret.Base32(),
		OTPAuthURI: domain.OTPAuthURI(twoFactorIssuer, user.Email, secret),
	}, nil
}

func (s *TwoFactorService) ConfirmEnrollment(ctx context.Context, userID uuid.UUID, code string) ([]string, error) {
	setup, err := s.pendingSetup(ctx, userID)
	if err != nil {
		return nil, err
	}
	secret, err := s.openSecret(userID, setup)
	if err != nil {
		return nil, err
	}
	now := s.clock.Now().UTC()
	step, ok := domain.VerifyTOTP(secret, code, now, setup.LastUsedStep)
	if !ok {
		return nil, domain.ErrTwoFactorInvalid
	}
	codes, err := domain.GenerateRecoveryCodes(domain.RecoveryCodeCount)
	if err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to create recovery codes", err)
	}

	err = s.tx.InTx(ctx, func(ctx context.Context) error {
		enabled, err := s.repo.MarkEnabled(ctx, userID, now, step)
		if err != nil {
			return err
		}
		if !enabled {
			return domain.ErrTwoFactorAlreadyEnabled
		}
		if err := s.repo.ReplaceRecoveryCodes(ctx, userID, hashCodes(codes)); err != nil {
			return err
		}
		return s.publish(ctx, userID, EventTwoFactorEnabled, now, map[string]any{"recovery_codes": len(codes)})
	})
	if err != nil {
		return nil, err
	}
	return codes, nil
}

func (s *TwoFactorService) Disable(ctx context.Context, userID uuid.UUID, password string, in port.CompleteLoginInput) error {
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return domain.ErrIncorrectPassword
	}
	setup, err := s.enabledSetup(ctx, userID)
	if err != nil {
		return err
	}

	now := s.clock.Now().UTC()
	return s.tx.InTx(ctx, func(ctx context.Context) error {
		if err := s.verifySecondFactor(ctx, userID, setup, in, now); err != nil {
			return err
		}
		if err := s.repo.Delete(ctx, userID); err != nil {
			return err
		}
		return s.publish(ctx, userID, EventTwoFactorDisabled, now, nil)
	})
}

func (s *TwoFactorService) RegenerateRecoveryCodes(ctx context.Context, userID uuid.UUID, code string) ([]string, error) {
	setup, err := s.enabledSetup(ctx, userID)
	if err != nil {
		return nil, err
	}
	secret, err := s.openSecret(userID, setup)
	if err != nil {
		return nil, err
	}
	codes, err := domain.GenerateRecoveryCodes(domain.RecoveryCodeCount)
	if err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to create recovery codes", err)
	}

	now := s.clock.Now().UTC()
	err = s.tx.InTx(ctx, func(ctx context.Context) error {
		step, ok := domain.VerifyTOTP(secret, code, now, setup.LastUsedStep)
		if !ok {
			return domain.ErrTwoFactorInvalid
		}
		advanced, err := s.repo.AdvanceLastUsedStep(ctx, userID, step)
		if err != nil {
			return err
		}
		if !advanced {
			return domain.ErrTwoFactorInvalid
		}
		if err := s.repo.ReplaceRecoveryCodes(ctx, userID, hashCodes(codes)); err != nil {
			return err
		}
		return s.publish(ctx, userID, EventRecoveryCodesRegenerated, now, map[string]any{"recovery_codes": len(codes)})
	})
	if err != nil {
		return nil, err
	}
	return codes, nil
}

func (s *TwoFactorService) StartChallenge(ctx context.Context, userID uuid.UUID) (uuid.UUID, time.Time, error) {
	expiresAt := s.clock.Now().UTC().Add(domain.ChallengeTTL)
	id, err := s.repo.CreateChallenge(ctx, userID, expiresAt)
	if err != nil {
		return uuid.Nil, time.Time{}, err
	}
	return id, expiresAt, nil
}

// CompleteChallenge checks the second factor of a login challenge. A wrong code is counted on the challenge
// outside the transaction, so the count survives the rollback of the failed attempt. The challenge is
// consumed together with the successful verification, so it cannot be used twice.
func (s *TwoFactorService) CompleteChallenge(ctx context.Context, challengeID, userID uuid.UUID, in port.CompleteLoginInput) (*domain.User, error) {
	now := s.clock.Now().UTC()
	challenge, err := s.repo.FindChallenge(ctx, challengeID)
	if err != nil {
		return nil, err
	}
	if !challengeUsable(challenge, userID, now) {
		return nil, domain.ErrChallengeExpired
	}
	setup, err := s.enabledSetup(ctx, userID)
	if err != nil {
		if errors.Is(err, domain.ErrTwoFactorNotEnabled) {
			return nil, domain.ErrChallengeExpired
		}
		return nil, err
	}

	err = s.tx.InTx(ctx, func(ctx context.Context) error {
		if err := s.verifySecondFactor(ctx, userID, setup, in, now); err != nil {
			return err
		}
		consumed, err := s.repo.ConsumeChallenge(ctx, challengeID, now, domain.ChallengeMaxAttempts)
		if err != nil {
			return err
		}
		if !consumed {
			return domain.ErrChallengeExpired
		}
		return nil
	})
	if errors.Is(err, domain.ErrTwoFactorInvalid) {
		if recErr := s.repo.RecordFailedChallengeAttempt(ctx, challengeID, domain.ChallengeMaxAttempts, now); recErr != nil {
			return nil, recErr
		}
		return nil, err
	}
	if err != nil {
		return nil, err
	}

	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !user.IsActive() {
		return nil, domain.ErrChallengeExpired
	}
	return user, nil
}

// challengeUsable reports whether a challenge still accepts codes for the user.
func challengeUsable(c *port.LoginChallenge, userID uuid.UUID, now time.Time) bool {
	return c != nil && c.UserID == userID && c.ConsumedAt == nil &&
		now.Before(c.ExpiresAt) && c.FailedAttempts < domain.ChallengeMaxAttempts
}

// verifySecondFactor accepts either a TOTP code or a recovery code. It runs inside the caller's transaction.
func (s *TwoFactorService) verifySecondFactor(ctx context.Context, userID uuid.UUID, setup *port.TwoFactorSetup,
	in port.CompleteLoginInput, now time.Time) error {
	switch {
	case in.Code != "":
		secret, err := s.openSecret(userID, setup)
		if err != nil {
			return err
		}
		step, ok := domain.VerifyTOTP(secret, in.Code, now, setup.LastUsedStep)
		if !ok {
			return domain.ErrTwoFactorInvalid
		}
		advanced, err := s.repo.AdvanceLastUsedStep(ctx, userID, step)
		if err != nil {
			return err
		}
		if !advanced {
			return domain.ErrTwoFactorInvalid
		}
		return nil
	case in.RecoveryCode != "":
		return s.consumeRecoveryCode(ctx, userID, in.RecoveryCode, now)
	default:
		return domain.ErrTwoFactorRequired
	}
}

// consumeRecoveryCode compares the digest with every unused code in constant time and marks the match used.
func (s *TwoFactorService) consumeRecoveryCode(ctx context.Context, userID uuid.UUID, code string, now time.Time) error {
	digest := domain.HashRecoveryCode(code)
	records, err := s.repo.ListUnusedRecoveryCodes(ctx, userID)
	if err != nil {
		return err
	}
	var matched *port.RecoveryCodeRecord
	for i := range records {
		if subtle.ConstantTimeCompare(records[i].CodeHash, digest) == 1 {
			matched = &records[i]
		}
	}
	if matched == nil {
		return domain.ErrTwoFactorInvalid
	}
	marked, err := s.repo.MarkRecoveryCodeUsed(ctx, userID, matched.ID, now)
	if err != nil {
		return err
	}
	if !marked {
		return domain.ErrTwoFactorInvalid
	}
	return s.publish(ctx, userID, EventRecoveryCodeUsed, now, nil)
}

// pendingSetup returns the setup of a user whose enrollment is started but not confirmed.
func (s *TwoFactorService) pendingSetup(ctx context.Context, userID uuid.UUID) (*port.TwoFactorSetup, error) {
	setup, err := s.repo.FindSetup(ctx, userID)
	if err != nil {
		return nil, err
	}
	if setup == nil {
		return nil, domain.ErrTwoFactorNotEnrolled
	}
	if setup.EnabledAt != nil {
		return nil, domain.ErrTwoFactorAlreadyEnabled
	}
	return setup, nil
}

// enabledSetup returns the setup of a user with the second factor enabled.
func (s *TwoFactorService) enabledSetup(ctx context.Context, userID uuid.UUID) (*port.TwoFactorSetup, error) {
	setup, err := s.repo.FindSetup(ctx, userID)
	if err != nil {
		return nil, err
	}
	if setup == nil || setup.EnabledAt == nil {
		return nil, domain.ErrTwoFactorNotEnabled
	}
	return setup, nil
}

func (s *TwoFactorService) openSecret(userID uuid.UUID, setup *port.TwoFactorSetup) (domain.TOTPSecret, error) {
	raw, err := s.cipher.Open(setup.SecretEncrypted, secretAAD(userID))
	if err != nil {
		return nil, apperror.Wrap(apperror.KindInternal, "failed to read two-factor secret", err)
	}
	return domain.TOTPSecret(raw), nil
}

func (s *TwoFactorService) publish(ctx context.Context, userID uuid.UUID, eventType string, at time.Time, payload map[string]any) error {
	actor := userID
	if err := s.publisher.Publish(ctx, events.Event{
		Type:       eventType,
		EntityType: "user",
		EntityID:   &actor,
		ActorID:    &actor,
		Payload:    payload,
		CreatedAt:  at,
	}); err != nil {
		return fmt.Errorf("two-factor: publish %s: %w", eventType, err)
	}
	return nil
}

func hashCodes(codes []string) [][]byte {
	out := make([][]byte, 0, len(codes))
	for _, c := range codes {
		out = append(out, domain.HashRecoveryCode(c))
	}
	return out
}
