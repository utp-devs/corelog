// Package port declares the note use case contract, the repository and the narrow ports of other modules.
package port

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/note/domain"
)

// ListQuery filters and pages the notes of one owner. Pinned, IssueID and ProjectID are optional filters.
// After is the keyset position of the last item of the previous page (nil for the first page).
// Limit is the number of items to return; the repository is asked for one extra row to detect a next page.
type ListQuery struct {
	Query     string
	Tag       string
	Pinned    *bool
	IssueID   *uuid.UUID
	ProjectID *uuid.UUID
	After     *Position
	Limit     int
}

// Position is the sort key of a note in the list order (pinned first, then updated_at, then id; all descending).
type Position struct {
	Pinned    bool      `json:"p"`
	UpdatedAt time.Time `json:"u"`
	ID        uuid.UUID `json:"i"`
}

// ListPage is one page of notes. NextCursor is empty when there are no more notes.
type ListPage struct {
	Items      []domain.Note
	NextCursor string
}

// TagCount is one tag of the owner with the number of live notes that carry it.
type TagCount struct {
	Tag   string `json:"tag"`
	Count int    `json:"count"`
}

// SearchHit is a note matched by the search provider.
type SearchHit struct {
	ID        uuid.UUID
	Title     string
	UpdatedAt time.Time
}

// Repository persists notes. Every method is scoped to the owner: a note of another user is reported as
// domain.ErrNotFound.
type Repository interface {
	Insert(ctx context.Context, note domain.Note) (domain.Note, error)
	// Update writes the content fields of a live note owned by note.OwnerID.
	Update(ctx context.Context, note domain.Note) error
	Get(ctx context.Context, ownerID, id uuid.UUID) (domain.Note, error)
	List(ctx context.Context, ownerID uuid.UUID, query ListQuery) ([]domain.Note, error)
	// SoftDelete marks a live note as deleted at the given time.
	SoftDelete(ctx context.Context, ownerID, id uuid.UUID, at time.Time) error
	ListTags(ctx context.Context, ownerID uuid.UUID) ([]TagCount, error)
	// SearchOwn returns up to limit live notes whose title or body contains query, most recently updated first.
	SearchOwn(ctx context.Context, ownerID uuid.UUID, query string, limit int) ([]SearchHit, error)
}

// LinkAccess answers whether a user may browse an issue or a project. It is the only way this module learns
// about other modules' data, and it returns false for missing, deleted and hidden elements alike.
type LinkAccess interface {
	CanBrowseIssue(ctx context.Context, userID, issueID uuid.UUID) (bool, error)
	CanBrowseProject(ctx context.Context, userID, projectID uuid.UUID) (bool, error)
}

// SearchProvider exposes the owner's notes to the global search.
type SearchProvider interface {
	// SearchOwn matches title and body of the user's live notes. An empty query returns no hits.
	// A limit of zero or less takes the default; values above the maximum are clamped.
	SearchOwn(ctx context.Context, userID uuid.UUID, query string, limit int) ([]SearchHit, error)
}

// Service is the note use case. Every method acts on behalf of actorID and only sees that user's notes.
type Service interface {
	SearchProvider
	Create(ctx context.Context, actorID uuid.UUID, fields domain.Fields) (domain.Note, error)
	Update(ctx context.Context, actorID, id uuid.UUID, patch domain.Patch) (domain.Note, error)
	Delete(ctx context.Context, actorID, id uuid.UUID) error
	Get(ctx context.Context, actorID, id uuid.UUID) (domain.Note, error)
	List(ctx context.Context, actorID uuid.UUID, query ListQuery) (ListPage, error)
	SetPinned(ctx context.Context, actorID, id uuid.UUID, pinned bool) (domain.Note, error)
	ListTags(ctx context.Context, actorID uuid.UUID) ([]TagCount, error)
}
