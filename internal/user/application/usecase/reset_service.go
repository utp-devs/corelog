package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/platform/database"
	"github.com/jhonsferg/corelog/internal/platform/events"
	"github.com/jhonsferg/corelog/internal/platform/mailer"
	"github.com/jhonsferg/corelog/internal/user/application/port"
	"github.com/jhonsferg/corelog/internal/user/domain"
)

// Event types published by the recovery flows.
const (
	EventPasswordResetRequested = "auth.password_reset_requested"
	EventPasswordReset          = "auth.password_reset"
	EventPasswordChanged        = "auth.password_changed"
	EventEmailVerified          = "auth.email_verified"
)

// RecoveryService implements the password reset and email verification flows.
//
// Extension point: a reset by SMS code would reuse the hashed one time codes of the phone verification
// (PhoneVerificationRepository) as a second way to prove ownership and end in the same ResetPassword core.
type RecoveryService struct {
	users         port.Repository
	sessions      port.SessionRepository
	resets        port.TokenStore
	verifications port.TokenStore
	lockouts      port.LockoutStore
	tx            database.TxManager
	mail          mailer.Mailer
	publisher     events.Publisher
	clock         clock.Clock
	baseURL       string
	log           *slog.Logger
	// dispatch runs the email delivery. The default is a detached goroutine so the response time of the
	// forgot endpoint does not depend on the SMTP relay (nor on whether the account exists).
	dispatch func(func())
}

var _ port.RecoveryService = (*RecoveryService)(nil)

// NewRecoveryService wires the service. Tokens, mailer and base URL are required for the email flows.
func NewRecoveryService(users port.Repository, sessions port.SessionRepository, resets, verifications port.TokenStore,
	lockouts port.LockoutStore, tx database.TxManager, mail mailer.Mailer, publisher events.Publisher,
	clk clock.Clock, baseURL string, log *slog.Logger) *RecoveryService {
	if log == nil {
		log = slog.Default()
	}
	return &RecoveryService{
		users: users, sessions: sessions, resets: resets, verifications: verifications, lockouts: lockouts,
		tx: tx, mail: mail, publisher: publisher, clock: clk, baseURL: baseURL, log: log,
		dispatch: func(f func()) { go f() },
	}
}

// WithSyncDispatch delivers emails inline (tests).
func (s *RecoveryService) WithSyncDispatch() *RecoveryService {
	s.dispatch = func(f func()) { f() }
	return s
}

func (s *RecoveryService) inTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if s.tx == nil {
		return fn(ctx)
	}
	return s.tx.InTx(ctx, fn)
}

func (s *RecoveryService) send(msg mailer.Message) {
	s.dispatch(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := s.mail.Send(ctx, msg); err != nil {
			s.log.Error("email delivery failed", slog.String("error", err.Error()))
		}
	})
}

// RequestPasswordReset issues a reset token for the account and mails the link. Unknown, deactivated and
// failing cases are indistinguishable for the caller.
func (s *RecoveryService) RequestPasswordReset(ctx context.Context, email string) error {
	email = strings.TrimSpace(email)
	plain, hash, err := domain.NewToken()
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to create reset token", err)
	}
	user, err := s.users.FindByEmail(ctx, email)
	if err != nil || !user.IsActive() {
		// Same generic outcome; only infrastructure errors are worth a log line.
		if err != nil && !isNotFound(err) {
			s.log.Error("password reset lookup failed", slog.String("error", err.Error()))
		}
		return nil
	}
	now := s.clock.Now()
	if err := s.resets.Replace(ctx, user.ID, hash, now.Add(domain.PasswordResetTTL), now); err != nil {
		s.log.Error("password reset token store failed", slog.String("error", err.Error()))
		return nil
	}
	s.publish(ctx, user.ID, EventPasswordResetRequested, now)
	s.send(resetEmail(user.Email, user.FirstName, s.baseURL, plain))
	return nil
}

// ResetPassword applies a new password. A weak password leaves the token usable.
func (s *RecoveryService) ResetPassword(ctx context.Context, in port.ResetPasswordInput) error {
	if err := domain.CheckPasswordConfirmation(in.Password, in.PasswordConfirmation, "password_confirmation"); err != nil {
		return err
	}
	if strings.TrimSpace(in.Token) == "" {
		return domain.ErrResetTokenInvalid
	}
	hash := domain.HashToken(in.Token)
	now := s.clock.Now()
	// The token is only inspected first, so a weak password does not burn it.
	userID, found, err := s.resets.Find(ctx, hash, now)
	if err != nil {
		return err
	}
	if !found {
		return domain.ErrResetTokenInvalid
	}
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		if isNotFound(err) {
			return domain.ErrResetTokenInvalid
		}
		return err
	}
	if !user.IsActive() {
		return domain.ErrResetTokenInvalid
	}
	if err := domain.ValidatePassword(in.Password, user.Email, "password"); err != nil {
		return err
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to hash password", err)
	}
	return s.inTx(ctx, func(ctx context.Context) error {
		// Consume is the atomic gate: of two concurrent resets with the same token only one passes.
		if _, ok, err := s.resets.Consume(ctx, hash, now); err != nil {
			return err
		} else if !ok {
			return domain.ErrResetTokenInvalid
		}
		user.ChangePasswordHash(string(hashed))
		if err := s.users.Update(ctx, user); err != nil {
			return err
		}
		if err := s.sessions.RevokeExcept(ctx, user.ID, nil, now); err != nil {
			return err
		}
		if s.lockouts != nil {
			if err := s.lockouts.Delete(ctx, LockoutKey(user.Email)); err != nil {
				return err
			}
		}
		return s.publishErr(ctx, user.ID, EventPasswordReset, now)
	})
}

// RequestEmailVerification mails a verification link to the address of the user.
func (s *RecoveryService) RequestEmailVerification(ctx context.Context, userID uuid.UUID) error {
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return err
	}
	return s.SendVerification(ctx, user)
}

// SendVerification issues a verification token for the user and mails the link.
func (s *RecoveryService) SendVerification(ctx context.Context, user *domain.User) error {
	if user.EmailVerifiedAt != nil {
		return domain.ErrEmailAlreadyVerified
	}
	plain, hash, err := domain.NewToken()
	if err != nil {
		return apperror.Wrap(apperror.KindInternal, "failed to create verification token", err)
	}
	now := s.clock.Now()
	if err := s.verifications.Replace(ctx, user.ID, hash, now.Add(domain.EmailVerificationTTL), now); err != nil {
		return err
	}
	s.send(verificationEmail(user.Email, user.FirstName, s.baseURL, plain))
	return nil
}

// VerifyEmail consumes a verification token and marks the address as verified.
func (s *RecoveryService) VerifyEmail(ctx context.Context, token string) error {
	if strings.TrimSpace(token) == "" {
		return domain.ErrVerificationTokenInvalid
	}
	return s.inTx(ctx, func(ctx context.Context) error {
		now := s.clock.Now()
		userID, found, err := s.verifications.Consume(ctx, domain.HashToken(token), now)
		if err != nil {
			return err
		}
		if !found {
			return domain.ErrVerificationTokenInvalid
		}
		user, err := s.users.FindByID(ctx, userID)
		if err != nil {
			if isNotFound(err) {
				return domain.ErrVerificationTokenInvalid
			}
			return err
		}
		at := now
		user.EmailVerifiedAt = &at
		user.UpdatedAt = now
		if err := s.users.Update(ctx, user); err != nil {
			return err
		}
		return s.publishErr(ctx, user.ID, EventEmailVerified, now)
	})
}

func (s *RecoveryService) publish(ctx context.Context, userID uuid.UUID, typ string, at time.Time) {
	if err := s.publishErr(ctx, userID, typ, at); err != nil {
		s.log.Error("audit event failed", slog.String("type", typ), slog.String("error", err.Error()))
	}
}

func (s *RecoveryService) publishErr(ctx context.Context, userID uuid.UUID, typ string, at time.Time) error {
	if s.publisher == nil {
		return nil
	}
	id := userID
	if err := s.publisher.Publish(ctx, events.Event{
		Type: typ, EntityType: "user", EntityID: &id, ActorID: &id, Payload: map[string]any{}, CreatedAt: at,
	}); err != nil {
		return fmt.Errorf("recovery: publish %s: %w", typ, err)
	}
	return nil
}

func isNotFound(err error) bool { return apperror.KindOf(err) == apperror.KindNotFound }
