package port

import (
	"context"

	"github.com/google/uuid"
)

// LoginInfo describes a login that just opened a session.
type LoginInfo struct {
	UserID    uuid.UUID
	SessionID uuid.UUID
	UserAgent string
	// IP is the address of the caller as resolved by the trusted proxy rules; it is masked before storage.
	IP string
}

// SessionEvents is the narrow port to the security notifications: the alert of a new login and the live
// closing of revoked sessions. Failures never fail the request, so the methods return nothing.
type SessionEvents interface {
	NewLogin(ctx context.Context, info LoginInfo)
	// SessionRevoked tells the open streams of the session that it was closed.
	SessionRevoked(ctx context.Context, userID, sessionID uuid.UUID)
	// OthersRevoked tells the open streams of every session but keepID (uuid.Nil: all) that they were closed.
	OthersRevoked(ctx context.Context, userID, keepID uuid.UUID)
}
