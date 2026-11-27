package domain

import (
	"crypto/sha256"
	"crypto/subtle"
	"time"

	"github.com/google/uuid"
)

// Phone verification policy.
const (
	PhoneCodeDigits      = 6
	PhoneCodeTTL         = 10 * time.Minute
	PhoneCodeMaxAttempts = 5
	PhoneCodeMaxSends    = 3
	PhoneCodeSendWindow  = time.Hour
)

// PhoneVerification is one SMS code issued for a number. Only the SHA-256 of the code is kept.
type PhoneVerification struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	E164       string
	CodeHash   []byte
	Attempts   int
	ExpiresAt  time.Time
	ConsumedAt *time.Time
	CreatedAt  time.Time
}

// HashPhoneCode returns the SHA-256 digest stored for a code.
func HashPhoneCode(code string) []byte {
	sum := sha256.Sum256([]byte(code))
	return sum[:]
}

// Matches compares a candidate code with the stored digest in constant time.
func (v *PhoneVerification) Matches(code string) bool {
	return subtle.ConstantTimeCompare(v.CodeHash, HashPhoneCode(code)) == 1
}

// Usable reports whether the code can still be tried at the given instant.
func (v *PhoneVerification) Usable(now time.Time) bool {
	return v.ConsumedAt == nil && now.Before(v.ExpiresAt)
}
