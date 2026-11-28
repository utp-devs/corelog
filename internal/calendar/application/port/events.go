package port

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/calendar/domain"
)

// Actions of the project authorizer used for project events.
const ActionEditIssue = "edit_issue"

// Viewer is the caller of a calendar operation. GlobalRole is the platform role ("admin" or "user").
type Viewer struct {
	UserID     uuid.UUID
	GlobalRole string
}

// PlatformAdmin reports whether the viewer is the platform administrator (support access).
func (v Viewer) PlatformAdmin() bool { return v.GlobalRole == "admin" }

// Kinds of the items of the merged calendar.
const (
	KindEvent          = "event"
	KindIssueDue       = "issue_due"
	KindIssueStart     = "issue_start"
	KindSprintStart    = "sprint_start"
	KindSprintEnd      = "sprint_end"
	KindVersionRelease = "version_release"
)

// DerivedQuery selects the dates derived from issues, sprints and versions. A nil Projects means the projects the
// viewer is a member of. Kinds lists the derived kinds wanted.
type DerivedQuery struct {
	Viewer   Viewer
	From, To time.Time
	Projects []uuid.UUID
	Kinds    []string
}

// EventRepository persists calendar events, their attendees and the feed token. Reads only return what the
// viewer can see.
type EventRepository interface {
	// OrganizationFor resolves the organization of a new event: the requested one when the user belongs to it,
	// else the user's first one.
	OrganizationFor(ctx context.Context, userID, requested uuid.UUID) (uuid.UUID, error)
	// ProjectOrganization returns the organization of a live project.
	ProjectOrganization(ctx context.Context, projectID uuid.UUID) (uuid.UUID, error)
	// OrgRole returns the role of the user in the organization, or "" when not a member.
	OrgRole(ctx context.Context, userID, organizationID uuid.UUID) (string, error)
	// MembersAmong returns the subset of userIDs that belong to the organization.
	MembersAmong(ctx context.Context, organizationID uuid.UUID, userIDs []uuid.UUID) ([]uuid.UUID, error)

	// Create stores the event with its attendees in one transaction and fills ID.
	Create(ctx context.Context, e *domain.Event) error
	// Update replaces the event fields and exceptions. When replaceAttendees is true the guest list becomes
	// e.Attendees (existing responses are kept for users that stay).
	Update(ctx context.Context, e *domain.Event, replaceAttendees bool) error
	Delete(ctx context.Context, id uuid.UUID, at time.Time) error
	// FindVisible returns the event with attendees, or domain.ErrEventNotFound when it does not exist or the
	// viewer cannot see it.
	FindVisible(ctx context.Context, viewer Viewer, id uuid.UUID) (*domain.Event, error)
	// ListVisible returns the visible events that may have an occurrence in [from, to) (recurring events are
	// returned as masters). A nil projects slice does not filter; otherwise project events outside it are dropped.
	ListVisible(ctx context.Context, viewer Viewer, from, to time.Time, projects []uuid.UUID) ([]domain.Event, error)
	// SetResponse stores the answer of a guest and reports whether the user is one.
	SetResponse(ctx context.Context, eventID, userID uuid.UUID, response domain.Response) (bool, error)
	// Derived lists the dates of issues, sprints and versions of the projects the viewer can browse.
	Derived(ctx context.Context, q DerivedQuery) ([]Event, error)

	// FeedInfo returns when the active feed token was created, or nil when the user has none.
	FeedInfo(ctx context.Context, userID uuid.UUID) (*time.Time, error)
	// ReplaceFeedToken stores the hash of a new token and revokes the previous one.
	ReplaceFeedToken(ctx context.Context, userID uuid.UUID, tokenHash string, at time.Time) error
	// RevokeFeedToken disables the feed and reports whether there was an active one.
	RevokeFeedToken(ctx context.Context, userID uuid.UUID, at time.Time) (bool, error)
	// UserByFeedToken returns the owner of an active token hash, or domain.ErrFeedNotFound.
	UserByFeedToken(ctx context.Context, tokenHash string) (uuid.UUID, error)
}

// EventInput is the body of a new event.
type EventInput struct {
	OrganizationID uuid.UUID
	ProjectID      *uuid.UUID
	Title          string
	Description    string
	Location       string
	StartsAt       time.Time
	EndsAt         time.Time
	AllDay         bool
	Timezone       string
	RecurrenceRule *string
	Visibility     domain.Visibility
	Color          string
	Attendees      []uuid.UUID
}

// OptString is a string that may be set, cleared (Set with nil Value) or left untouched (not Set).
type OptString struct {
	Set   bool
	Value *string
}

// OptUUID is the UUID counterpart of OptString.
type OptUUID struct {
	Set   bool
	Value *uuid.UUID
}

// EventPatch holds the fields to change; nil leaves a field as it is. With an occurrence only Title,
// Description, Location, Color, StartsAt and EndsAt can change.
type EventPatch struct {
	Title          *string
	Description    *string
	Location       *string
	StartsAt       *time.Time
	EndsAt         *time.Time
	AllDay         *bool
	Timezone       *string
	RecurrenceRule OptString
	Visibility     *domain.Visibility
	Color          *string
	ProjectID      OptUUID
	Attendees      *[]uuid.UUID
}

// EventView is an event with what the viewer can do with it.
type EventView struct {
	Event      domain.Event
	CanEdit    bool
	MyResponse domain.Response
}

// ItemsQuery asks for the merged calendar. Kinds empty means every kind; Projects empty means no project filter
// for events and the viewer's own projects for derived dates.
type ItemsQuery struct {
	From, To time.Time
	Projects []uuid.UUID
	Kinds    []string
}

// Item is one entry of the merged calendar: an event occurrence or a date derived from an issue, sprint or
// version. For all-day entries StartsAt and EndsAt are UTC midnights and EndsAt is exclusive.
type Item struct {
	Kind            string
	Title           string
	StartsAt        time.Time
	EndsAt          time.Time
	AllDay          bool
	SourceID        uuid.UUID
	ProjectID       *uuid.UUID
	ProjectKey      string
	Link            string
	Color           string
	Location        string
	Timezone        string
	Visibility      string
	Recurring       bool
	OccurrenceStart *time.Time
	CanEdit         bool
	MyResponse      domain.Response
}

// ItemsResult is the merged calendar. Truncated is true when a source hit its cap.
type ItemsResult struct {
	Items     []Item
	Truncated bool
}

// FeedToken is returned once, when the feed token is created.
type FeedToken struct {
	Token     string
	CreatedAt time.Time
}

// EventService is the event use case seen by the HTTP adapter.
type EventService interface {
	Create(ctx context.Context, viewer Viewer, in EventInput) (*EventView, error)
	Get(ctx context.Context, viewer Viewer, id uuid.UUID) (*EventView, error)
	// Update changes the whole event or, with occurrence set, one occurrence of a recurring event.
	Update(ctx context.Context, viewer Viewer, id uuid.UUID, occurrence *time.Time, patch EventPatch) (*EventView, error)
	// Delete removes the event or, with occurrence set, cancels one occurrence.
	Delete(ctx context.Context, viewer Viewer, id uuid.UUID, occurrence *time.Time) error
	Respond(ctx context.Context, viewer Viewer, id uuid.UUID, response domain.Response) error
	Items(ctx context.Context, viewer Viewer, q ItemsQuery) (ItemsResult, error)

	FeedInfo(ctx context.Context, userID uuid.UUID) (*time.Time, error)
	CreateFeed(ctx context.Context, userID uuid.UUID) (FeedToken, error)
	RevokeFeed(ctx context.Context, userID uuid.UUID) error
	// RenderFeed returns the ICS of the owner of the token.
	RenderFeed(ctx context.Context, token string) (string, error)
}
