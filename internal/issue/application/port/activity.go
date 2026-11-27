package port

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/issue/domain"
)

// ActivityFeed is the in-port that lists the activity of one issue: comments and history merged in one feed.
type ActivityFeed interface {
	ListActivityFeed(ctx context.Context, issueID, actorID uuid.UUID, globalRole string, query ActivityQuery) (*ActivityFeedPage, error)
}

// ActivityQuery is the validated input of the feed. Limit is only read when LimitGiven is true.
type ActivityQuery struct {
	Filter     string
	Cursor     string
	Limit      int
	LimitGiven bool
}

// ActivityFeedPage is one page of the feed. NextCursor is empty when there are no older entries.
type ActivityFeedPage struct {
	Entries    []ActivityEntry
	NextCursor string
}

// ActivityEntry is a comment or a history entry (all the changes of one patch).
type ActivityEntry struct {
	ID        uuid.UUID
	Kind      string
	At        time.Time
	ActorID   uuid.UUID
	ActorName string
	Comment   *ActivityComment
	Changes   []ActivityChange
}

// ActivityComment is the body of a comment entry.
type ActivityComment struct {
	ID        uuid.UUID
	Body      string
	UpdatedAt time.Time
	Edited    bool
}

// ActivityChange is one field change rendered for people: the field name is translated and ids are resolved
// to their current names.
type ActivityChange struct {
	Action string
	Field  string
	Label  string
	From   *string
	To     *string
}

// ActivityFeedReader is the out-port that reads the rows behind the feed. Every method works on one issue.
type ActivityFeedReader interface {
	// FeedRefs returns up to limit entries ordered newest first, after the cursor when it is not nil.
	FeedRefs(ctx context.Context, issueID uuid.UUID, filter string, after *domain.ActivityCursor, limit int) ([]FeedRef, error)
	// FeedContent loads the comments and activity rows of the given entries in a fixed number of statements.
	FeedContent(ctx context.Context, issueID uuid.UUID, refs []FeedRef) (FeedContent, error)
	// ReferenceNames returns the current names of catalog rows. kind is one of the ReferenceKind constants.
	ReferenceNames(ctx context.Context, kind string, ids []uuid.UUID) (map[uuid.UUID]string, error)
}

// Reference kinds understood by ReferenceNames.
const (
	ReferencePriority    = "priority"
	ReferenceIssueType   = "issue_type"
	ReferenceLabel       = "label"
	ReferenceCustomField = "custom_field"
)

// FeedRef identifies one entry of the feed: a comment id or the batch id of a history entry.
type FeedRef struct {
	Kind string
	ID   uuid.UUID
	At   time.Time
}

// FeedContent holds the loaded rows: comments by id and activity rows by entry id (batch id or activity id).
type FeedContent struct {
	Comments   map[uuid.UUID]*domain.Comment
	Activities map[uuid.UUID][]*domain.Activity
}

// IssueLabelRepository reads and writes the labels of an issue. Labels are project-scoped.
type IssueLabelRepository interface {
	// IssueLabelIDs returns the ids of the labels of the issue, sorted.
	IssueLabelIDs(ctx context.Context, issueID uuid.UUID) ([]uuid.UUID, error)
	// ResolveLabelNames returns the id of each name in the project, creating the missing labels. The result keeps
	// the order of names.
	ResolveLabelNames(ctx context.Context, projectID uuid.UUID, names []string) ([]uuid.UUID, error)
	// ReplaceIssueLabels sets the labels of the issue.
	ReplaceIssueLabels(ctx context.Context, issueID uuid.UUID, labelIDs []uuid.UUID) error
}
