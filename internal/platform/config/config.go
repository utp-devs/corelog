package config

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
)

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Auth     AuthConfig
	Legal    LegalConfig
	Security SecurityConfig
	Mail     MailConfig
	// Retention drives the data lifecycle jobs.
	Retention RetentionConfig
}

type ServerConfig struct {
	Port           string   `env:"SERVER_PORT" envDefault:"8080"`
	Env            string   `env:"APP_ENV" envDefault:"development"`
	AllowedOrigins []string `env:"CORS_ALLOWED_ORIGIN" envDefault:"http://localhost:5173" envSeparator:","`
	// CorsAllowedOrigins (CORS_ALLOWED_ORIGINS) takes precedence over the legacy singular variable.
	CorsAllowedOrigins []string `env:"CORS_ALLOWED_ORIGINS" envSeparator:","`
	// ReadHeaderTimeout bounds the time a client has to send the request headers (slow header attacks).
	ReadHeaderTimeout time.Duration `env:"SERVER_READ_HEADER_TIMEOUT" envDefault:"5s"`
	ReadTimeout       time.Duration `env:"SERVER_READ_TIMEOUT" envDefault:"10s"`
	WriteTimeout      time.Duration `env:"SERVER_WRITE_TIMEOUT" envDefault:"10s"`
	// IdleTimeout closes keep-alive connections that stay unused.
	IdleTimeout     time.Duration `env:"SERVER_IDLE_TIMEOUT" envDefault:"120s"`
	ShutdownTimeout time.Duration `env:"SERVER_SHUTDOWN_TIMEOUT" envDefault:"15s"`
}

type DatabaseConfig struct {
	Host            string        `env:"DB_HOST" envDefault:"localhost"`
	Port            string        `env:"DB_PORT" envDefault:"5432"`
	User            string        `env:"DB_USER" envDefault:"corelog"`
	Password        string        `env:"DB_PASSWORD" envDefault:"corelog"`
	Name            string        `env:"DB_NAME" envDefault:"corelog"`
	SSLMode         string        `env:"DB_SSLMODE" envDefault:"disable"`
	MaxOpenConns    int           `env:"DB_MAX_OPEN_CONNS" envDefault:"25"`
	MaxIdleConns    int           `env:"DB_MAX_IDLE_CONNS" envDefault:"25"`
	ConnMaxLifetime time.Duration `env:"DB_CONN_MAX_LIFETIME" envDefault:"5m"`
}

func (d DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		d.Host, d.Port, d.User, d.Password, d.Name, d.SSLMode,
	)
}

type AuthConfig struct {
	JWTSecret     string        `env:"JWT_SECRET" envDefault:"change-me-in-production"`
	JWTExpiration time.Duration `env:"JWT_EXPIRATION" envDefault:"24h"`
	// TwoFactorEncryptionKey is a base64 encoded 32 byte key for the stored TOTP secrets. When empty, a key is
	// derived from JWTSecret in development; outside development the server refuses to start without it.
	TwoFactorEncryptionKey string `env:"TWO_FACTOR_ENCRYPTION_KEY"`
}

// LegalConfig controls how the legal consent requirement is enforced after login.
type LegalConfig struct {
	// Enforcement is "warn" (default: pending consents are only reported) or "block" (authenticated calls other
	// than the consent endpoints answer 403 legal.consent_pending until the user accepts).
	Enforcement string `env:"LEGAL_ENFORCEMENT" envDefault:"warn"`
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("config: parse: %w", err)
	}

	if err := cfg.Retention.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}
