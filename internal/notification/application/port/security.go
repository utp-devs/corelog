package port

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/notification/domain"
)

// SecurityAlerts is what the login flow calls once a session is open.
type SecurityAlerts interface {
	// NewLogin tells the user about a login from a device not seen in the last 30 days.
	NewLogin(ctx context.Context, login domain.NewLogin) error
}

// DeviceHistory remembers the devices of each account.
type DeviceHistory interface {
	// Touch records the login of a device and returns when it was last seen before (nil when never) and
	// whether the account had any device on record.
	Touch(ctx context.Context, userID uuid.UUID, fingerprint string, now time.Time) (lastSeen *time.Time, hasHistory bool, err error)
}

// ContactReader finds the email address of a user.
type ContactReader interface {
	Contacts(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]Contact, error)
}
