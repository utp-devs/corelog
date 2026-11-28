package main

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/jhonsferg/corelog/internal/audit"
	auditpostgres "github.com/jhonsferg/corelog/internal/audit/adapter/out/postgres"
	orgport "github.com/jhonsferg/corelog/internal/organization/application/port"
	orgdomain "github.com/jhonsferg/corelog/internal/organization/domain"
	"github.com/jhonsferg/corelog/internal/platform/clock"
	"github.com/jhonsferg/corelog/internal/platform/config"
	"github.com/jhonsferg/corelog/internal/platform/database"
	"github.com/jhonsferg/corelog/internal/platform/events"
	"github.com/jhonsferg/corelog/internal/platform/httpserver"
	"github.com/jhonsferg/corelog/internal/platform/jobs"
	"github.com/jhonsferg/corelog/internal/platform/mailer"
	appmw "github.com/jhonsferg/corelog/internal/platform/middleware"
	"github.com/jhonsferg/corelog/internal/platform/ratelimit"
	"github.com/jhonsferg/corelog/internal/user"
	userhttp "github.com/jhonsferg/corelog/internal/user/adapter/in/http"
	userpostgres "github.com/jhonsferg/corelog/internal/user/adapter/out/postgres"
	userport "github.com/jhonsferg/corelog/internal/user/application/port"
	userdomain "github.com/jhonsferg/corelog/internal/user/domain"
)

// auditOrgAccess lets only owners and admins of an organization read its audit log. The organization service
// answers not found for non-members and forbidden for lower roles.
type auditOrgAccess struct {
	svc orgport.Service
}

func (a auditOrgAccess) RequireAdmin(ctx context.Context, userID, orgID uuid.UUID) error {
	_, err := a.svc.RequireRole(ctx, userID, orgID, orgdomain.RoleAdmin)
	return err
}

// newMailer returns the SMTP mailer when SMTP_HOST is set, otherwise the log mailer, which only prints the
// action link in development.
func newMailer(cfg *config.Config, log *slog.Logger) mailer.Mailer {
	if cfg.Mail.Host == "" {
		return mailer.Log{Logger: log, ShowBody: cfg.Server.Env == "development"}
	}
	return mailer.NewSMTP(mailer.SMTPConfig{
		Host: cfg.Mail.Host, Port: cfg.Mail.Port, User: cfg.Mail.User, Password: cfg.Mail.Password, From: cfg.Mail.From,
	})
}

func trustedProxies(cfg *config.Config, log *slog.Logger) *appmw.TrustedProxies {
	tp, bad := appmw.ParseTrustedProxies(cfg.Security.TrustedProxies)
	for _, entry := range bad {
		log.Warn("ignoring invalid TRUSTED_PROXIES entry", "entry", entry)
	}
	return tp
}

// routerOptions maps the security settings to the router hardening options.
func routerOptions(cfg *config.Config, log *slog.Logger) httpserver.RouterOptions {
	return httpserver.RouterOptions{
		HSTS:             cfg.Security.HSTS,
		MaxJSONBodyBytes: cfg.Security.MaxJSONBodyBytes,
		TrustedProxies:   trustedProxies(cfg, log),
		Logger:           log,
	}
}

// wireUserSecurity enables password recovery, email verification, rate limiting and the login lockout on the
// user module. With RATE_LIMIT_DISABLED=true outside production no limiter and no lockout is installed.
func wireUserSecurity(cfg *config.Config, db *sqlx.DB, log *slog.Logger, module *user.Module, userRepo userport.Repository,
	sessions userport.SessionRepository, tx database.TxManager, publisher events.Publisher) {
	if cfg.Server.IsProduction() && cfg.Security.RateLimitDisabled {
		log.Warn("RATE_LIMIT_DISABLED is ignored in production")
	}
	lockouts := userpostgres.NewLockoutRepository(db)
	recovery := user.NewRecoveryService(userRepo, sessions, userpostgres.NewPasswordResetTokens(db),
		userpostgres.NewEmailVerificationTokens(db), lockouts, tx, newMailer(cfg, log), publisher,
		clock.RealClock{}, cfg.Security.AppPublicURL, log)
	module.WithEmailVerification(cfg.Security.RequireEmailVerification, recovery)

	sec := userhttp.Security{Recovery: recovery}
	if cfg.LimitsEnabled() {
		s := cfg.Security
		policy := userdomain.DefaultLockoutPolicy()
		if s.LockoutThreshold > 0 {
			policy.Threshold = s.LockoutThreshold
		}
		if s.LockoutDuration > 0 {
			policy.Duration = s.LockoutDuration
			policy.Window = s.LockoutDuration
		}
		sec.Guard = user.NewLoginGuard(lockouts, policy, clock.RealClock{}, publisher)
		clk := clock.RealClock{}
		sec.Limits = &userhttp.RateLimits{
			Login:         ratelimit.New(ratelimit.PerMinute(s.RateLimitLoginPerMin), clk),
			LoginTwoFact:  ratelimit.New(ratelimit.PerMinute(s.RateLimitLoginPerMin), clk),
			Register:      ratelimit.New(ratelimit.PerHour(s.RateLimitRegisterPerHour), clk),
			Forgot:        ratelimit.New(ratelimit.PerHour(s.RateLimitForgotPerHour), clk),
			ForgotAccount: ratelimit.New(ratelimit.PerHour(s.RateLimitForgotPerHour), clk),
			Reset:         ratelimit.New(ratelimit.PerMinute(s.RateLimitLoginPerMin), clk),
			Phone:         ratelimit.New(ratelimit.PerHour(s.RateLimitPhonePerHour), clk),
		}
		tp := trustedProxies(cfg, log)
		sec.ClientIP = func(r *http.Request) string { return tp.ClientIP(r) }
	}
	module.Handler.WithSecurity(sec)
}

// newAuditModule wires the audit hexagon (consumer, retention, read API).
func newAuditModule(cfg *config.Config, db *sqlx.DB, orgs orgport.Service) *audit.Module {
	return audit.NewModule(auditpostgres.NewRepository(db), auditOrgAccess{svc: orgs},
		cfg.Security.AuditRetentionDays, cfg.Auth.JWTSecret, clock.RealClock{})
}

// startSecurityRuntime runs the audit consumer over the domain events outbox (persisted cursor, at least
// once, idempotent inserts) until ctx is done. Its periodic cleanups run on the job scheduler.
func startSecurityRuntime(ctx context.Context, cfg *config.Config, db *sqlx.DB, log *slog.Logger) error {
	module := audit.NewModule(auditpostgres.NewRepository(db), nil, cfg.Security.AuditRetentionDays, cfg.Auth.JWTSecret, clock.RealClock{})
	dispatcher, err := events.NewDispatcher(db, []events.Consumer{module.Consumer}, events.Options{Logger: log})
	if err != nil {
		return err
	}
	return dispatcher.Run(ctx, time.Second)
}

func init() { registerJobs(registerSecurityJobs) }

// registerSecurityJobs adds the audit retention purge and the cleanup of expired tokens and lockouts.
func registerSecurityJobs(s *jobs.Scheduler, d jobDeps) {
	clk := clock.RealClock{}
	module := audit.NewModule(auditpostgres.NewRepository(d.DB), nil, d.Cfg.Security.AuditRetentionDays, d.Cfg.Auth.JWTSecret, clk)
	mustRegister(s, d.Log, "audit-retention", "every 24h", func(ctx context.Context) error { return module.RetentionJob(ctx) })
	lockouts := userpostgres.NewLockoutRepository(d.DB)
	resets, verifications := userpostgres.NewPasswordResetTokens(d.DB), userpostgres.NewEmailVerificationTokens(d.DB)
	mustRegister(s, d.Log, "security-cleanup", "every 1h", func(ctx context.Context) error {
		cutoff := clk.Now().Add(-24 * time.Hour)
		if _, err := lockouts.PurgeStale(ctx, cutoff); err != nil {
			return err
		}
		if _, err := resets.PurgeExpired(ctx, cutoff); err != nil {
			return err
		}
		_, err := verifications.PurgeExpired(ctx, cutoff)
		return err
	})
}
