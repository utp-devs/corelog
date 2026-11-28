package port

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/notification/domain"
)

// StoredScheme is a scheme row.
type StoredScheme struct {
	Scheme    domain.Scheme
	UpdatedAt time.Time
}

// SchemeRepository stores the notification schemes. A nil projectID addresses the organization default.
type SchemeRepository interface {
	// Get returns nil when no scheme is stored for the scope.
	Get(ctx context.Context, orgID uuid.UUID, projectID *uuid.UUID) (*StoredScheme, error)
	Save(ctx context.Context, orgID uuid.UUID, projectID *uuid.UUID, scheme domain.Scheme, updatedBy uuid.UUID, now time.Time) (StoredScheme, error)
	// Delete reports whether a row was removed.
	Delete(ctx context.Context, orgID uuid.UUID, projectID *uuid.UUID) (bool, error)
	// ProjectOrg returns the organization of a project, or uuid.Nil when it does not exist.
	ProjectOrg(ctx context.Context, projectID uuid.UUID) (uuid.UUID, error)
}

// IssueFacts are the people and texts around an issue.
type IssueFacts struct {
	IssueID          uuid.UUID
	OrgID            uuid.UUID
	ProjectID        uuid.UUID
	Key              string
	Summary          string
	ReporterID       uuid.UUID
	AssigneeID       uuid.UUID
	WatcherIDs       []uuid.UUID
	ComponentLeadIDs []uuid.UUID
	ProjectLeadID    uuid.UUID
}

// ProjectFacts are the people and texts around a project.
type ProjectFacts struct {
	ProjectID uuid.UUID
	OrgID     uuid.UUID
	Name      string
	LeadID    uuid.UUID
}

// Contact is how to reach a user.
type Contact struct {
	ID     uuid.UUID
	Name   string
	Email  string
	Active bool
}

// FactsSource reads, through narrow queries, what the engine needs to find recipients.
type FactsSource interface {
	// Issue returns nil when the issue does not exist or was deleted.
	Issue(ctx context.Context, issueID uuid.UUID) (*IssueFacts, error)
	// Project returns nil when the project does not exist.
	Project(ctx context.Context, projectID uuid.UUID) (*ProjectFacts, error)
	TeamMembers(ctx context.Context, teamIDs []uuid.UUID) (map[uuid.UUID][]uuid.UUID, error)
	RoleMembers(ctx context.Context, projectID uuid.UUID, roles []string) (map[string][]uuid.UUID, error)
	Contacts(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]Contact, error)
}

// DigestItem is one notice waiting for the daily summary of a user.
type DigestItem struct {
	ID        int64
	UserID    uuid.UUID
	Event     domain.Event
	IssueKey  string
	Summary   string
	ActorName string
	Link      string
	CreatedAt time.Time
}

// DeliveryStore holds the delivery bookkeeping: dedupe claims and digest items. Methods join the transaction
// found in the context.
type DeliveryStore interface {
	// Claim records that (key, user) was delivered and reports false when it already was.
	Claim(ctx context.Context, key string, userID uuid.UUID, now time.Time) (bool, error)
	AddDigestItem(ctx context.Context, item DigestItem) error
	UsersWithDigest(ctx context.Context, limit int) ([]uuid.UUID, error)
	DigestItems(ctx context.Context, userID uuid.UUID, limit int) ([]DigestItem, error)
	MarkDigested(ctx context.Context, ids []int64, at time.Time) error
	PurgeDeliveries(ctx context.Context, before time.Time) (int64, error)
}

// Outbox statuses.
const (
	OutboxPending = "pending"
	OutboxSent    = "sent"
	OutboxDead    = "dead"
)

// EmailMessage is a message to enqueue. IdempotencyKey may be empty.
type EmailMessage struct {
	UserID         uuid.UUID
	To             string
	Kind           string
	Subject        string
	Text           string
	HTML           string
	IdempotencyKey string
	NotBefore      time.Time
}

// OutboxEntry is a stored message.
type OutboxEntry struct {
	ID            uuid.UUID
	UserID        *uuid.UUID
	To            string
	Kind          string
	Subject       string
	Text          string
	HTML          string
	Status        string
	Attempts      int
	NextAttemptAt time.Time
	LastError     string
	CreatedAt     time.Time
	SentAt        *time.Time
}

// Outbox is the email queue.
type Outbox interface {
	// Enqueue stores the message and reports false when its idempotency key already exists.
	Enqueue(ctx context.Context, msg EmailMessage, now time.Time) (bool, error)
	// ClaimDue leases up to limit due messages (FOR UPDATE SKIP LOCKED): their next attempt moves to now+lease.
	ClaimDue(ctx context.Context, now time.Time, limit int, lease time.Duration) ([]OutboxEntry, error)
	MarkSent(ctx context.Context, id uuid.UUID, now time.Time) error
	// MarkFailed records a failed attempt. A nil next marks the message dead.
	MarkFailed(ctx context.Context, id uuid.UUID, attempts int, next *time.Time, lastError string, now time.Time) error
	List(ctx context.Context, status string, limit, offset int) ([]OutboxEntry, int, error)
	// Retry puts a dead message back in the queue. It reports false when the id is not a dead message.
	Retry(ctx context.Context, id uuid.UUID, now time.Time) (bool, error)
}

// Email is the data of a rendered message.
type Email struct {
	Event      domain.Event
	Greeting   string
	IssueKey   string
	Summary    string
	ActorName  string
	Detail     string
	Link       string
	Preference string
	FilterName string
	Lines      []EmailLine
	Total      int
}

// EmailLine is one row of a digest or filter message.
type EmailLine struct {
	Label  string
	Text   string
	Detail string
	Link   string
}

// Rendered is a message ready to enqueue.
type Rendered struct {
	Subject string
	Text    string
	HTML    string
}

// EmailRenderer builds the Spanish messages.
type EmailRenderer interface {
	Notice(Email) Rendered
	Digest(Email) Rendered
	FilterSubscription(Email) Rendered
}

// DueFilter is a filter subscription whose delivery is pending.
type DueFilter struct {
	SubscriptionID uuid.UUID
	UserID         uuid.UUID
	FilterName     string
	FilterID       uuid.UUID
	LastSentAt     *time.Time
}

// FilterRow is one issue of a filter result.
type FilterRow struct {
	Key     string
	Summary string
	Status  string
}

// FilterSubscriptions is the narrow view of the saved filter module the subscription job needs.
type FilterSubscriptions interface {
	Due(ctx context.Context) ([]DueFilter, error)
	// Run executes the filter as its subscriber and returns the first rows and the total.
	Run(ctx context.Context, due DueFilter, limit int) ([]FilterRow, int, error)
	MarkDelivered(ctx context.Context, subscriptionID uuid.UUID) error
}

// Notice is an event ready to be delivered to the people its scheme names.
type Notice struct {
	Event     domain.Event
	Kind      string
	OrgID     uuid.UUID
	ProjectID uuid.UUID
	IssueID   uuid.UUID
	ActorID   uuid.UUID
	ActorName string
	Detail    string
	// Message and Link override the generated in-app text and link.
	Message string
	Link    string
	// DedupKey makes the delivery idempotent per recipient. Empty means no dedupe.
	DedupKey string
	// Explicit replaces the people of a recipient type (mentioned users, the author a reply answers, the new assignee).
	Explicit map[domain.RecipientType][]uuid.UUID
	// SkipVisibility skips the browse check of the recipients (the caller already decided who may be told).
	SkipVisibility bool
	// Only limits the delivery to one user (the per-person notices of the comment consumer).
	Only uuid.UUID
}
