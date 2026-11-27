package main

import (
	"fmt"
	"log/slog"

	"github.com/jhonsferg/corelog/internal/platform/config"
	"github.com/jhonsferg/corelog/internal/platform/crypto"
)

// twoFactorCipher returns the cipher for the stored TOTP secrets. The key comes from
// TWO_FACTOR_ENCRYPTION_KEY. Without it, a key is derived from the JWT secret and a warning is logged once;
// run() refuses to start outside development in that case.
func twoFactorCipher(cfg *config.Config, log *slog.Logger) (*crypto.Cipher, error) {
	var key []byte
	if cfg.Auth.TwoFactorEncryptionKey != "" {
		parsed, err := crypto.ParseKey(cfg.Auth.TwoFactorEncryptionKey)
		if err != nil {
			return nil, fmt.Errorf("TWO_FACTOR_ENCRYPTION_KEY must be 32 bytes encoded as base64: %w", err)
		}
		key = parsed
	} else {
		derived, err := crypto.DeriveKey([]byte(cfg.Auth.JWTSecret), "corelog two-factor secret encryption")
		if err != nil {
			return nil, err
		}
		key = derived
		log.Warn("TWO_FACTOR_ENCRYPTION_KEY is not set: two-factor secrets use a key derived from JWT_SECRET")
	}
	cipher, err := crypto.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("two-factor cipher: %w", err)
	}
	return cipher, nil
}

// requireTwoFactorKey fails fast outside development when the encryption key is not configured.
func requireTwoFactorKey(cfg *config.Config) error {
	if cfg.Server.Env != "development" && cfg.Auth.TwoFactorEncryptionKey == "" {
		return fmt.Errorf("TWO_FACTOR_ENCRYPTION_KEY is required when APP_ENV is %q", cfg.Server.Env)
	}
	return nil
}
