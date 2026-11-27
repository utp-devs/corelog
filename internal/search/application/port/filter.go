package port

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/search/domain"
)

// FilterView is a filter as one viewer sees it.
type FilterView struct {
	domain.Filter
	OwnerName       string
	Access          domain.Access
	IsFavorite      bool
	HasSubscription bool
}

// ShareView is a share with the display name of its target.
type ShareView struct {
	domain.Share
	TargetName string
}

// FilterScope narrows a filter listing.
type FilterScope string

const (
	ScopeAll      FilterScope = "all"
	ScopeMine     FilterScope = "mine"
	ScopeShared   FilterScope = "shared"
	ScopeFavorite FilterScope = "favorite"
)

// FilterListQuery is a page of the filters visible to a viewer.
type FilterListQuery struct {
	Scope    FilterScope
	Text     string
	Page     int
	PageSize int
}

// FilterRepository stores filters, shares and subscriptions. Reads take the viewer and return only what the
// viewer may see; a filter that is not visible is reported as domain.ErrFilterNotFound.
type FilterRepository interface {
	// OrganizationFor returns the organization a new filter belongs to: requested when the user is a member,
	// otherwise the first organization of the user.
	OrganizationFor(ctx context.Context, userID, requested uuid.UUID) (uuid.UUID, error)
	Create(ctx context.Context, ownerID, orgID uuid.UUID, fields domain.FilterFields, now time.Time) (uuid.UUID, error)
	Get(ctx context.Context, viewerID, id uuid.UUID) (*FilterView, error)
	List(ctx context.Context, viewerID uuid.UUID, q FilterListQuery) ([]FilterView, int, error)
	Update(ctx context.Context, id uuid.UUID, fields domain.FilterFields, now time.Time) error
	Delete(ctx context.Context, id uuid.UUID) error
	Shares(ctx context.Context, id uuid.UUID) ([]ShareView, error)
	ReplaceShares(ctx context.Context, id, orgID uuid.UUID, shares []domain.Share, now time.Time) error
	GetSubscription(ctx context.Context, filterID, userID uuid.UUID) (*domain.Subscription, error)
	UpsertSubscription(ctx context.Context, filterID, userID uuid.UUID, schedule domain.Schedule, now time.Time) (domain.Subscription, error)
	DeleteSubscription(ctx context.Context, filterID, userID uuid.UUID) error
	// MarkSubscriptionSent records a delivery. It reports domain.ErrSubscriptionNotFound for an unknown id.
	MarkSubscriptionSent(ctx context.Context, id uuid.UUID, at time.Time) error
	// ListSubscriptions pages through every subscription ordered by id, after the given id.
	ListSubscriptions(ctx context.Context, after uuid.UUID, limit int) ([]domain.Subscription, error)
}

// DueSubscription is a subscription whose delivery is pending, with the filter it delivers.
type DueSubscription struct {
	Subscription domain.Subscription
	Filter       domain.Filter
}
