// Package port declares the legal hexagon boundaries.
package port

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/legal/domain"
)

// Repository stores documents and consents. Methods join the transaction bound to the context, if any.
type Repository interface {
	// ListCurrent returns, per slug, the highest version already in effect at now. Bodies are left empty.
	ListCurrent(ctx context.Context, now time.Time) ([]domain.Document, error)
	// GetCurrent returns the current version of slug with its body, or domain.ErrDocumentNotFound.
	GetCurrent(ctx context.Context, slug domain.Slug, now time.Time) (*domain.Document, error)
	// GetVersion returns an exact version with its body, or domain.ErrDocumentNotFound.
	GetVersion(ctx context.Context, slug domain.Slug, version int) (*domain.Document, error)
	// ListAll returns every version of every document, newest first, without bodies.
	ListAll(ctx context.Context) ([]domain.Document, error)
	// Publish inserts d as the next version of its slug, filling ID, Version, EffectiveFrom and PublishedAt.
	Publish(ctx context.Context, d *domain.Document, now time.Time) error

	// ActiveConsents lists the non-revoked consents of the user.
	ActiveConsents(ctx context.Context, userID uuid.UUID) ([]domain.Accepted, error)
	// SaveConsent stores the consent; it reports false when the same version was already actively accepted.
	SaveConsent(ctx context.Context, c domain.Consent) (bool, error)
	// RevokeConsent revokes the active consents of slug; it reports false when there was none.
	RevokeConsent(ctx context.Context, userID uuid.UUID, slug domain.Slug, at time.Time) (bool, error)
	// ListConsents is the audit list, newest first. A nil userID lists every user.
	ListConsents(ctx context.Context, userID *uuid.UUID, limit int) ([]domain.Consent, error)
}

// RecordInput is a consent submission.
type RecordInput struct {
	Accepted  []domain.Ref
	Source    domain.Source
	IP        string
	UserAgent string
}

// PublishInput is a new document version.
type PublishInput struct {
	Slug         domain.Slug
	Title        string
	Summary      string
	BodyMarkdown string
	Required     *bool
}

// Service is the in-port of the legal hexagon.
type Service interface {
	ListCurrent(ctx context.Context) ([]domain.Document, error)
	// GetDocument returns the current version when version is 0, otherwise the exact version.
	GetDocument(ctx context.Context, slug domain.Slug, version int) (*domain.Document, error)

	Status(ctx context.Context, userID uuid.UUID) (domain.Status, error)
	Pending(ctx context.Context, userID uuid.UUID) ([]domain.Pending, error)
	// Record stores the accepted current versions and returns the new status. Re-accepting is idempotent.
	Record(ctx context.Context, userID uuid.UUID, in RecordInput) (domain.Status, error)
	// ValidateRequired fails with legal.consent_required unless refs cover every required current document.
	ValidateRequired(ctx context.Context, refs []domain.Ref) error
	// Revoke withdraws the optional communications consent.
	Revoke(ctx context.Context, userID uuid.UUID, slug domain.Slug) error

	AdminList(ctx context.Context) ([]domain.Document, error)
	Publish(ctx context.Context, actorID uuid.UUID, in PublishInput) (*domain.Document, error)
	Audit(ctx context.Context, userID *uuid.UUID, limit int) ([]domain.Consent, error)
}
