// Package port declares the webhook service and the narrow ports it needs.
package port

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/events"
	"github.com/jhonsferg/corelog/internal/webhook/domain"
)

// Stored is a webhook with its encrypted secret.
type Stored struct {
	domain.Webhook
	SecretEncrypted []byte
}

// DueDelivery is a delivery ready to be sent with the data needed to sign and post it.
type DueDelivery struct {
	domain.Delivery
	URL             string
	SecretEncrypted []byte
}

// Result is the outcome of recording one attempt.
type Result struct {
	Status         domain.DeliveryStatus
	Attempts       int
	NextAttemptAt  *time.Time
	ResponseStatus *int
	Excerpt        string
	DurationMS     int
}

// Repository stores webhooks and deliveries.
type Repository interface {
	Create(ctx context.Context, w *Stored) error
	Get(ctx context.Context, orgID, id uuid.UUID) (Stored, error)
	List(ctx context.Context, orgID uuid.UUID, projectID *uuid.UUID) ([]domain.Webhook, error)
	Update(ctx context.Context, w domain.Webhook) error
	UpdateSecret(ctx context.Context, orgID, id uuid.UUID, sealed []byte, at time.Time) error
	Delete(ctx context.Context, orgID, id uuid.UUID) error
	// ProjectInOrg reports whether a live project belongs to the organization.
	ProjectInOrg(ctx context.Context, orgID, projectID uuid.UUID) (bool, error)
	// Matching returns the enabled webhooks of the organization that apply to a project (organization-wide ones
	// and the ones of that project) and subscribe to the event type.
	Matching(ctx context.Context, orgID uuid.UUID, projectID *uuid.UUID, eventType string) ([]domain.Webhook, error)
	// InsertDelivery stores a delivery. A second delivery of the same webhook and event id is ignored (ok false).
	InsertDelivery(ctx context.Context, d *domain.Delivery) (bool, error)
	ListDeliveries(ctx context.Context, webhookID uuid.UUID, limit int) ([]domain.Delivery, error)
	GetDelivery(ctx context.Context, webhookID, id uuid.UUID) (domain.Delivery, error)
	// ClaimDue returns up to limit deliveries due at now and pushes their next attempt by lease so another
	// worker does not send them at the same time.
	ClaimDue(ctx context.Context, now time.Time, limit int, lease time.Duration) ([]DueDelivery, error)
	// Webhook returns one webhook by id for the delivery worker, regardless of its enabled flag.
	StoredByID(ctx context.Context, id uuid.UUID) (Stored, error)
	RecordResult(ctx context.Context, id uuid.UUID, r Result) error
}

// OrgAccess decides who manages the webhooks of an organization: non-members get not found, members below
// admin get forbidden.
type OrgAccess interface {
	RequireAdmin(ctx context.Context, userID, orgID uuid.UUID) error
}

// SecretBox seals and opens stored secrets (AES-GCM).
type SecretBox interface {
	Seal(plain string) ([]byte, error)
	Open(sealed []byte) (string, error)
}

// SendRequest is one HTTP POST.
type SendRequest struct {
	URL     string
	Body    []byte
	Headers map[string]string
}

// SendResult is the response of an endpoint. Err is set when no response was obtained (blocked address, DNS,
// connection, timeout).
type SendResult struct {
	Status   int
	Body     []byte
	Duration time.Duration
	Err      error
}

// Sender posts to an endpoint with network protections.
type Sender interface {
	Send(ctx context.Context, req SendRequest) SendResult
}

// EventInfo describes an event for building the payload.
type EventInfo struct {
	OrgID      uuid.UUID
	Project    *ProjectInfo
	Issue      *IssueInfo
	ActorName  string
	ActorEmail string
}

// ProjectInfo is the project part of a payload.
type ProjectInfo struct {
	ID   uuid.UUID
	Key  string
	Name string
}

// IssueInfo is the issue part of a payload.
type IssueInfo struct {
	ID         uuid.UUID
	Key        string
	Summary    string
	Status     string
	Priority   string
	IssueType  string
	AssigneeID *uuid.UUID
	ReporterID uuid.UUID
}

// EventDescriber loads the organization, project, issue and actor of an event. Events whose organization cannot
// be resolved return ok false.
type EventDescriber interface {
	Describe(ctx context.Context, e events.Event) (EventInfo, bool, error)
}

// IssueMatcher decides whether an issue matches a CoreQL filter.
type IssueMatcher interface {
	Matches(ctx context.Context, projectID, issueID uuid.UUID, query string) (bool, error)
	// Validate checks the syntax of a filter text.
	Validate(ctx context.Context, query string) error
}

// CreateInput creates a webhook.
type CreateInput struct {
	Name        string
	URL         string
	Events      []string
	FilterQuery *string
	ProjectID   *uuid.UUID
	Enabled     *bool
}

// UpdateInput changes a webhook; nil fields keep their value. ClearFilter removes the filter.
type UpdateInput struct {
	Name        *string
	URL         *string
	Events      []string
	FilterQuery *string
	Enabled     *bool
}

// Service is the in-port. The secret is returned in clear only by Create and RotateSecret.
type Service interface {
	Create(ctx context.Context, actorID, orgID uuid.UUID, in CreateInput) (domain.Webhook, string, error)
	List(ctx context.Context, actorID, orgID uuid.UUID, projectID *uuid.UUID) ([]domain.Webhook, error)
	Get(ctx context.Context, actorID, orgID, id uuid.UUID) (domain.Webhook, error)
	Update(ctx context.Context, actorID, orgID, id uuid.UUID, in UpdateInput) (domain.Webhook, error)
	Delete(ctx context.Context, actorID, orgID, id uuid.UUID) error
	RotateSecret(ctx context.Context, actorID, orgID, id uuid.UUID) (string, error)
	// Test sends a ping synchronously and returns the recorded delivery.
	Test(ctx context.Context, actorID, orgID, id uuid.UUID) (domain.Delivery, error)
	Deliveries(ctx context.Context, actorID, orgID, id uuid.UUID, limit int) ([]domain.Delivery, error)
	// Redeliver queues a copy of a delivery payload for sending.
	Redeliver(ctx context.Context, actorID, orgID, id, deliveryID uuid.UUID) (domain.Delivery, error)
}
