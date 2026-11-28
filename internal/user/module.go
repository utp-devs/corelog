package user

import (
	"github.com/jhonsferg/corelog/internal/platform/mailer"
	"github.com/jhonsferg/corelog/internal/user/domain"
	"log/slog"
	"time"

	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/platform/database"
	"github.com/jhonsferg/corelog/internal/platform/events"
	httpadapter "github.com/jhonsferg/corelog/internal/user/adapter/in/http"
	"github.com/jhonsferg/corelog/internal/user/application/port"
	"github.com/jhonsferg/corelog/internal/user/application/usecase"
)

type Module struct {
	Service port.Service
	users   *usecase.UserService
	Handler *httpadapter.Handler
}

// NewModule wires the user use cases. tx and detacher make account deactivation atomic and remove the
// closed account from organization, team and project memberships.
func NewModule(repo port.Repository, sessions port.SessionRepository, tx database.TxManager,
	detacher port.MembershipDetacher, jwtSecret string, jwtExpiration time.Duration) *Module {
	service := usecase.NewUserService(repo, sessions).WithAccountClosure(tx, detacher)
	handler := httpadapter.NewHandler(service, jwtSecret, jwtExpiration)

	return &Module{
		users:   service,
		Service: service,
		Handler: handler,
	}
}

// WithPhoneVerification enables SMS verification of phone numbers: repo stores the hashed codes and sender
// delivers them (the default is sms.NoopSender; the provider adapter plugs in here).
func (m *Module) WithPhoneVerification(repo port.PhoneVerificationRepository, sender port.SMSSender) *Module {
	m.users.WithPhoneVerification(repo, sender)
	return m
}

// NewTwoFactorService wires the TOTP second factor. The cipher encrypts the stored secrets; the publisher
// records enable, disable and recovery code events inside the transaction of each change.
func NewTwoFactorService(users port.Repository, repo port.TwoFactorRepository, cipher port.SecretCipher,
	tx database.TxManager, publisher events.Publisher, clk clock.Clock) port.TwoFactorService {
	return usecase.NewTwoFactorService(users, repo, cipher, tx, publisher, clk)
}

// RecoveryService is the password reset and email verification service. It also implements
// usecase.EmailVerifier, so it can be handed to WithEmailVerification.
type RecoveryService = usecase.RecoveryService

// NewRecoveryService wires the password reset and email verification flows. baseURL is the public address of
// the web application, used to build the links of the emails.
func NewRecoveryService(users port.Repository, sessions port.SessionRepository, resets, verifications port.TokenStore,
	lockouts port.LockoutStore, tx database.TxManager, mail mailer.Mailer, publisher events.Publisher,
	clk clock.Clock, baseURL string, log *slog.Logger) *RecoveryService {
	return usecase.NewRecoveryService(users, sessions, resets, verifications, lockouts, tx, mail, publisher, clk, baseURL, log)
}

// NewLoginGuard wires the per-account failed login lockout. Thresholds of zero disable the lockout.
func NewLoginGuard(store port.LockoutStore, policy domain.LockoutPolicy, clk clock.Clock, publisher events.Publisher) port.LoginGuard {
	return usecase.NewLoginGuard(store, policy, clk, publisher)
}

// WithEmailVerification makes login require a verified email address when required is true and sends the
// verification link to new accounts through verifier (which may be nil).
func (m *Module) WithEmailVerification(required bool, verifier usecase.EmailVerifier) *Module {
	m.users.WithEmailVerification(required, verifier)
	return m
}
