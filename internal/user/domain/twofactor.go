package domain

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"math/big"
	"net/url"
	"strings"
	"time"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// TOTP parameters (RFC 6238 with HMAC-SHA1, the values every common authenticator app expects).
const (
	TOTPDigits     = 6
	TOTPPeriod     = 30 * time.Second
	TOTPWindow     = 1
	TOTPSecretSize = 20 // 160 bits

	RecoveryCodeCount  = 10
	RecoveryCodeLength = 10
	// RecoveryCodeAlphabet leaves out look-alike characters (0, 1, I, L, O, U) so codes can be copied by hand.
	RecoveryCodeAlphabet = "ABCDEFGHJKMNPQRSTVWXYZ23456789"

	// ChallengeMaxAttempts is the number of wrong codes a login challenge accepts before it is invalidated.
	ChallengeMaxAttempts = 5
	// ChallengeTTL is how long a login challenge stays valid after a correct password.
	ChallengeTTL = 5 * time.Minute
)

var (
	ErrTwoFactorAlreadyEnabled = apperror.WithCode(apperror.KindConflict, "auth.two_factor_already_enabled", "La autenticacion en dos pasos ya esta activada")
	ErrTwoFactorNotEnrolled    = apperror.WithCode(apperror.KindConflict, "auth.two_factor_not_enrolled", "Primero debes iniciar la configuracion de la autenticacion en dos pasos")
	ErrTwoFactorNotEnabled     = apperror.WithCode(apperror.KindConflict, "auth.two_factor_not_enabled", "La autenticacion en dos pasos no esta activada")
	ErrTwoFactorRequired       = apperror.WithCode(apperror.KindValidation, "auth.two_factor_required", "Introduce el codigo de tu aplicacion o un codigo de recuperacion")
	ErrTwoFactorInvalid        = apperror.WithCode(apperror.KindValidation, "auth.two_factor_invalid", "El codigo no es valido o ya fue utilizado")
	ErrChallengeExpired        = apperror.WithCode(apperror.KindUnauthorized, "auth.challenge_expired", "La verificacion ha caducado: inicia sesion de nuevo")
)

// TOTPSecret is a shared secret for a time-based one-time password. It is only held in memory; the
// persistence layer stores it encrypted.
type TOTPSecret []byte

// NewTOTPSecret draws a fresh 160-bit secret from the system random source.
func NewTOTPSecret() (TOTPSecret, error) {
	secret := make([]byte, TOTPSecretSize)
	if _, err := rand.Read(secret); err != nil {
		return nil, fmt.Errorf("totp: random secret: %w", err)
	}
	return secret, nil
}

// Base32 returns the secret in the unpadded base32 form that authenticator apps accept.
func (s TOTPSecret) Base32() string {
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(s)
}

// ParseTOTPSecret decodes a base32 secret produced by Base32.
func ParseTOTPSecret(encoded string) (TOTPSecret, error) {
	raw, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(encoded))
	if err != nil {
		return nil, fmt.Errorf("totp: decode secret: %w", err)
	}
	return TOTPSecret(raw), nil
}

// StepOf returns the RFC 6238 time step (counter) for an instant.
func StepOf(t time.Time) int64 {
	return t.Unix() / int64(TOTPPeriod/time.Second)
}

// TOTPAt computes the code for one time step (RFC 6238 section 4 with HMAC-SHA1 and 6 digits).
func TOTPAt(secret TOTPSecret, step int64) string {
	var counter [8]byte
	binary.BigEndian.PutUint64(counter[:], uint64(step))

	mac := hmac.New(sha1.New, secret)
	mac.Write(counter[:])
	sum := mac.Sum(nil)

	offset := sum[len(sum)-1] & 0x0f
	bin := binary.BigEndian.Uint32(sum[offset:offset+4]) & 0x7fffffff
	mod := uint32(1)
	for i := 0; i < TOTPDigits; i++ {
		mod *= 10
	}
	return fmt.Sprintf("%0*d", TOTPDigits, bin%mod)
}

// VerifyTOTP checks a user supplied code against the steps within TOTPWindow of now. It returns the
// matched step. A step at or below lastUsedStep is rejected so that a code cannot be replayed. The
// comparison is constant time.
func VerifyTOTP(secret TOTPSecret, code string, now time.Time, lastUsedStep *int64) (int64, bool) {
	code = strings.ReplaceAll(code, " ", "")
	if len(code) != TOTPDigits || !isDigits(code) {
		return 0, false
	}
	current := StepOf(now)
	matched := int64(-1)
	for delta := int64(-TOTPWindow); delta <= TOTPWindow; delta++ {
		step := current + delta
		if step < 0 {
			continue
		}
		candidate := TOTPAt(secret, step)
		if subtle.ConstantTimeCompare([]byte(candidate), []byte(code)) == 1 {
			matched = step
		}
	}
	if matched < 0 {
		return 0, false
	}
	if lastUsedStep != nil && matched <= *lastUsedStep {
		return 0, false
	}
	return matched, true
}

func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// OTPAuthURI builds the otpauth:// URI that authenticator apps read from a QR code.
func OTPAuthURI(issuer, account string, secret TOTPSecret) string {
	label := url.PathEscape(issuer) + ":" + url.PathEscape(account)
	query := url.Values{}
	query.Set("secret", secret.Base32())
	query.Set("issuer", issuer)
	query.Set("algorithm", "SHA1")
	query.Set("digits", fmt.Sprint(TOTPDigits))
	query.Set("period", fmt.Sprint(int(TOTPPeriod/time.Second)))
	return "otpauth://totp/" + label + "?" + query.Encode()
}

// GenerateRecoveryCodes draws n codes of RecoveryCodeLength characters from RecoveryCodeAlphabet. Values
// are drawn without modulo bias.
func GenerateRecoveryCodes(n int) ([]string, error) {
	alphabetLen := big.NewInt(int64(len(RecoveryCodeAlphabet)))
	codes := make([]string, 0, n)
	for i := 0; i < n; i++ {
		var b strings.Builder
		for b.Len() < RecoveryCodeLength {
			idx, err := rand.Int(rand.Reader, alphabetLen)
			if err != nil {
				return nil, fmt.Errorf("recovery code: random: %w", err)
			}
			b.WriteByte(RecoveryCodeAlphabet[idx.Int64()])
		}
		codes = append(codes, b.String())
	}
	return codes, nil
}

// NormalizeRecoveryCode removes separators and case differences a user may introduce while typing.
func NormalizeRecoveryCode(code string) string {
	code = strings.ToUpper(code)
	return strings.Map(func(r rune) rune {
		if r == '-' || r == ' ' {
			return -1
		}
		return r
	}, code)
}

// HashRecoveryCode returns the SHA-256 digest stored for a recovery code.
func HashRecoveryCode(code string) []byte {
	sum := sha256.Sum256([]byte(NormalizeRecoveryCode(code)))
	return sum[:]
}
