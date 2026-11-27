package port

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/search/domain/coreql"
)

// Viewer is the caller of a search. PlatformAdmin is the global administrator (support access: sees every
// live project). Timezone is the IANA name used to resolve relative dates; empty means UTC.
type Viewer struct {
	UserID        uuid.UUID
	PlatformAdmin bool
	Timezone      string
}

// CustomFieldDef is a custom field the viewer can query. IDs holds one id per organization that defines a
// field with this name.
type CustomFieldDef struct {
	Name string
	Kind string // text, paragraph, number, date, datetime, select, multiselect, checkbox, user, url, labels
	IDs  []uuid.UUID
}

// BuildError is a problem found while turning a validated query into SQL. Pos and Len are byte offsets.
type BuildError struct {
	Pos     int
	Len     int
	Code    string
	Message string
}

func (e *BuildError) Error() string { return fmt.Sprintf("coreql: %s", e.Message) }

// IssueQuery is a validated query plus everything needed to run it.
type IssueQuery struct {
	Viewer Viewer
	Query  *coreql.Query
	// Custom maps the lower-cased spelling of a custom field reference ("cf[story points]" or a bare name) to
	// its definition.
	Custom map[string]CustomFieldDef
	// ProjectID scopes the query to one project and gives currentProject() its value.
	ProjectID *uuid.UUID
	Now       time.Time
	Page      int
	PageSize  int
	WithTotal bool
	// DryRun builds the SQL (reporting build errors) without running it.
	DryRun bool
}

// IssueRef is a catalog entry shown on an issue.
type IssueRef struct {
	ID             uuid.UUID
	Name           string
	Icon           string
	Color          string
	HierarchyLevel int
	Category       string
}

// IssueRecord is one issue of a result page, shaped like an entry of the issue list.
type IssueRecord struct {
	ID                       uuid.UUID
	Key                      string
	ProjectID                uuid.UUID
	ProjectKey               string
	Summary                  string
	Description              string
	LegacyStatus             string
	Status                   *IssueRef
	IssueType                *IssueRef
	Priority                 *IssueRef
	Resolution               *IssueRef
	ResolvedAt               *time.Time
	ParentID                 *uuid.UUID
	StartDate                *string
	DueDate                  *string
	StoryPoints              *float64
	OriginalEstimateSeconds  *int64
	RemainingEstimateSeconds *int64
	Flagged                  bool
	VoteCount                int
	Rank                     string
	ClosedAt                 *time.Time
	ReporterID               uuid.UUID
	ReporterName             string
	AssigneeID               *uuid.UUID
	AssigneeName             *string
	Labels                   []string
	Custom                   map[string]any
	CreatedAt                time.Time
	UpdatedAt                time.Time
}

// IssuePage is a page of results. HasTotal is false when the count was not requested.
type IssuePage struct {
	Items    []IssueRecord
	Total    int
	HasTotal bool
}

// IssueSearcher runs validated queries. Implementations always restrict the result to the projects the
// viewer can browse, outside the query tree.
type IssueSearcher interface {
	SearchIssues(ctx context.Context, query IssueQuery) (IssuePage, error)
}

// CustomFieldSource lists the custom fields the viewer can query.
type CustomFieldSource interface {
	CustomFields(ctx context.Context, viewer Viewer) ([]CustomFieldDef, error)
}

// ViewerDirectory resolves viewer data that the token does not carry.
type ViewerDirectory interface {
	Timezone(ctx context.Context, userID uuid.UUID) (string, error)
}

// ValueSuggestion is one candidate value for a field.
type ValueSuggestion struct {
	Value  string
	Label  string
	Detail string
}

// SuggestSource lists candidate values of a field (users, statuses, labels, ...) the viewer can see.
type SuggestSource interface {
	SuggestValues(ctx context.Context, viewer Viewer, field string, prefix string, limit int) ([]ValueSuggestion, error)
}

// VersionKind says which version relation of an issue a condition refers to.
type VersionKind string

const (
	VersionFix     VersionKind = "fix"
	VersionAffects VersionKind = "affects"
)

// VersionSpec is one value of a version condition: a version name, a function (lower-cased, such as
// "releasedversions") or "has no version" when Empty is set.
type VersionSpec struct {
	Name     string
	Function string
	Empty    bool
}
