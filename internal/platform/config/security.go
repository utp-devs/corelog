package config

import "time"

// SecurityConfig groups abuse protection, hardening and retention settings.
type SecurityConfig struct {
	// RateLimitDisabled switches the rate limiter and the account lockout off for local and test stacks.
	// It is ignored when APP_ENV is production.
	RateLimitDisabled        bool `env:"RATE_LIMIT_DISABLED" envDefault:"false"`
	RateLimitLoginPerMin     int  `env:"RATE_LIMIT_LOGIN_PER_MIN" envDefault:"10"`
	RateLimitRegisterPerHour int  `env:"RATE_LIMIT_REGISTER_PER_HOUR" envDefault:"20"`
	RateLimitForgotPerHour   int  `env:"RATE_LIMIT_FORGOT_PER_HOUR" envDefault:"5"`
	RateLimitPhonePerHour    int  `env:"RATE_LIMIT_PHONE_PER_HOUR" envDefault:"10"`
	// LockoutThreshold is the number of failed logins of one account that starts a lockout.
	LockoutThreshold int           `env:"LOCKOUT_THRESHOLD" envDefault:"5"`
	LockoutDuration  time.Duration `env:"LOCKOUT_DURATION" envDefault:"15m"`
	// TrustedProxies are CIDRs or addresses whose X-Forwarded-For header is believed.
	TrustedProxies []string `env:"TRUSTED_PROXIES" envSeparator:","`
	// HSTS enables Strict-Transport-Security (set it when the API is served behind TLS).
	HSTS                     bool `env:"HSTS_ENABLED" envDefault:"false"`
	AuditRetentionDays       int  `env:"AUDIT_RETENTION_DAYS" envDefault:"365"`
	RequireEmailVerification bool `env:"REQUIRE_EMAIL_VERIFICATION" envDefault:"false"`
	// AppPublicURL builds the links of the emails (reset and verification).
	AppPublicURL string `env:"APP_PUBLIC_URL" envDefault:"http://localhost:5173"`
	// MaxJSONBodyBytes limits JSON request bodies.
	MaxJSONBodyBytes int64 `env:"MAX_JSON_BODY_BYTES" envDefault:"1048576"`
}

// MailConfig is the SMTP relay. With no host the log mailer is used. The password is never logged.
type MailConfig struct {
	Host     string `env:"SMTP_HOST"`
	Port     int    `env:"SMTP_PORT" envDefault:"587"`
	User     string `env:"SMTP_USER"`
	Password string `env:"SMTP_PASSWORD"`
	From     string `env:"SMTP_FROM" envDefault:"CoreLog <no-reply@localhost>"`
}

// Origins returns the CORS allow-list: CORS_ALLOWED_ORIGINS when set, otherwise the legacy variable.
func (s ServerConfig) Origins() []string {
	if len(s.CorsAllowedOrigins) > 0 {
		return s.CorsAllowedOrigins
	}
	return s.AllowedOrigins
}

// IsProduction reports whether the server runs with APP_ENV=production.
func (s ServerConfig) IsProduction() bool { return s.Env == "production" }

// LimitsEnabled reports whether the rate limiter and lockout are active.
func (c Config) LimitsEnabled() bool {
	return c.Server.IsProduction() || !c.Security.RateLimitDisabled
}
