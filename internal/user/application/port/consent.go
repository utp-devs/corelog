package port

import (
	"context"

	"github.com/google/uuid"
)

// ConsentRef names the version of a legal document the user accepted.
type ConsentRef struct {
	Slug    string
	Version int
}

// ConsentMeta is the evidence stored with a consent: where it was given and from which client.
type ConsentMeta struct {
	Source    string
	IP        string
	UserAgent string
}

// PendingConsent is a required legal document version the user has not accepted yet.
type PendingConsent struct {
	Slug    string
	Title   string
	Version int
}

// ConsentGate is the narrow port to the legal module. Validate rejects submissions that do not cover every
// required current document; Record stores the consents in the transaction bound to the context; Pending
// lists what a user still has to accept.
type ConsentGate interface {
	Validate(ctx context.Context, refs []ConsentRef) error
	Record(ctx context.Context, userID uuid.UUID, refs []ConsentRef, meta ConsentMeta) error
	Pending(ctx context.Context, userID uuid.UUID) ([]PendingConsent, error)
}
