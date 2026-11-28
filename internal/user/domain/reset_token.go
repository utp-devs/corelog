package domain

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// Lifetimes of the single use email tokens.
const (
	PasswordResetTTL     = time.Hour
	EmailVerificationTTL = 24 * time.Hour
	tokenBytes           = 32
)

var (
	// ErrResetTokenInvalid is returned for unknown, expired and already used reset tokens alike.
	ErrResetTokenInvalid = apperror.WithCode(apperror.KindValidation, "auth.reset_token_invalid",
		"El enlace de recuperación no es válido o ya venció: solicita uno nuevo")
	// ErrVerificationTokenInvalid is returned for unknown, expired and used verification tokens alike.
	ErrVerificationTokenInvalid = apperror.WithCode(apperror.KindValidation, "auth.verification_token_invalid",
		"El enlace de verificación no es válido o ya venció: solicita uno nuevo")
	// ErrEmailAlreadyVerified is returned when a verification is requested for a verified address.
	ErrEmailAlreadyVerified = apperror.WithCode(apperror.KindConflict, "auth.email_already_verified",
		"Tu correo electrónico ya está verificado")
	// ErrEmailNotVerified blocks login while REQUIRE_EMAIL_VERIFICATION is on and the address is unverified.
	ErrEmailNotVerified = apperror.WithCode(apperror.KindForbidden, "auth.email_not_verified",
		"Verifica tu correo electrónico para iniciar sesión: revisa tu bandeja de entrada")
	// ErrVerificationUnavailable is returned when no mailer is wired.
	ErrVerificationUnavailable = apperror.WithCode(apperror.KindConflict, "auth.verification_unavailable",
		"El envío de correos no está disponible")
)

// NewToken returns a URL-safe random token (32 bytes) and its SHA-256 hash. Only the hash is stored.
func NewToken() (plain string, hash []byte, err error) {
	raw := make([]byte, tokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, fmt.Errorf("generate token: %w", err)
	}
	plain = base64.RawURLEncoding.EncodeToString(raw)
	return plain, HashToken(plain), nil
}

// HashToken returns the SHA-256 digest of a token as presented by the client.
func HashToken(plain string) []byte {
	sum := sha256.Sum256([]byte(plain))
	return sum[:]
}
