package port

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/user/domain"
)

// SMSSender delivers a short text message. The provider integration lives behind this port; the default
// adapter only logs a masked destination. Implementations must never log the code outside development.
type SMSSender interface {
	SendVerificationCode(ctx context.Context, e164, code string) error
}

// PhoneVerificationRepository persists the SMS codes. Codes are stored hashed.
type PhoneVerificationRepository interface {
	// CreateWithinLimit stores the verification unless the user already requested max codes since the
	// given instant; in that case it returns domain.ErrPhoneSendLimit. The check and the insert are atomic
	// per user.
	CreateWithinLimit(ctx context.Context, v *domain.PhoneVerification, since time.Time, max int) error
	// FindLatestOpen returns the most recent unconsumed verification of the user for the number, or
	// domain.ErrPhoneCodeExpired when none exists.
	FindLatestOpen(ctx context.Context, userID uuid.UUID, e164 string) (*domain.PhoneVerification, error)
	// RegisterAttempt atomically increments the attempt counter and returns the new value.
	RegisterAttempt(ctx context.Context, id uuid.UUID) (int, error)
	// Consume marks the verification as used and discards the other open codes of the user.
	Consume(ctx context.Context, userID, id uuid.UUID, at time.Time) error
}

// PhoneChange is the phone part of a profile update: Clear removes the number; otherwise Country and
// Number replace it. Number may be national or carry the international prefix.
type PhoneChange struct {
	Clear   bool
	Country string
	Number  string
}

// PhoneVerificationReceipt tells the caller when the issued code expires.
type PhoneVerificationReceipt struct {
	ExpiresAt time.Time
	// Destination is the masked number the code was sent to.
	Destination string
}
