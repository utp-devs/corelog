// Package port declares the contracts of the report module: the use case offered to the HTTP adapter and
// the narrow ports it needs from persistence and from other modules.
package port

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/report/domain"
)

// ActionViewReports is the project permission that guards every report.
const ActionViewReports = "view_reports"

// Row caps that keep a report bounded.
const (
	MaxIssues         = 50000
	MaxHistoryRows    = 250000
	MaxFilterIssues   = 5000
	MaxWorklogRows    = 50000
	MaxScopeEventRows = 100000
)

// Authorizer decides whether an actor may run an action on a project. A hidden project is a not found
// error and a visible one without the permission a forbidden error.
type Authorizer interface {
	Authorize(ctx context.Context, action string, actorID uuid.UUID, globalRole string, projectID uuid.UUID) error
}

// IssueQuery selects live issues of one project. A nil pointer or false flag leaves the dimension open.
type IssueQuery struct {
	ProjectID uuid.UUID
	// OnlyIDs, when non-nil, restricts the result to those issues (an empty list matches nothing).
	OnlyIDs   *[]uuid.UUID
	SprintID  *uuid.UUID
	VersionID *uuid.UUID
	// CreatedBefore keeps issues created strictly before the instant.
	CreatedBefore *time.Time
	// TouchedSince keeps issues created or resolved at or after the instant.
	TouchedSince *time.Time
	OpenOnly     bool
}

// HistoryQuery selects status transitions of the live issues of a project made before an instant.
type HistoryQuery struct {
	ProjectID uuid.UUID
	OnlyIDs   *[]uuid.UUID
	Before    time.Time
}

// WorklogQuery selects live worklogs started inside [Lo, Hi).
type WorklogQuery struct {
	ProjectID uuid.UUID
	Lo, Hi    time.Time
	OnlyIDs   *[]uuid.UUID
	SprintID  *uuid.UUID
}

// Reader reads the history tables. Every method is project scoped and sends a constant number of
// statements whatever the amount of data. Readers return domain.ErrTooManyRows past the caps.
type Reader interface {
	Project(ctx context.Context, projectID uuid.UUID) (domain.ProjectInfo, error)
	// Sprint returns the sprint, or the active one (else the latest started) when id is nil.
	Sprint(ctx context.Context, projectID uuid.UUID, id *uuid.UUID) (domain.SprintInfo, error)
	// RecentSprints returns up to limit closed sprints, the latest first.
	RecentSprints(ctx context.Context, projectID uuid.UUID, limit int) ([]domain.SprintInfo, error)
	ScopeEvents(ctx context.Context, sprintID uuid.UUID) ([]domain.ScopeEvent, error)
	Version(ctx context.Context, projectID, versionID uuid.UUID) (domain.VersionInfo, error)
	Issues(ctx context.Context, q IssueQuery) ([]domain.IssueRow, error)
	StatusHistory(ctx context.Context, q HistoryQuery) ([]domain.StatusChange, error)
	Worklogs(ctx context.Context, q WorklogQuery) ([]domain.WorklogRow, error)
}

// IssueFilter resolves a CoreQL text to the ids of the issues of the project the actor may see. It
// returns at most limit+1 ids so the caller can detect an overly broad filter. Invalid text is a
// validation error.
type IssueFilter interface {
	MatchIssues(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID, text string, limit int) ([]uuid.UUID, error)
}

// SprintSummarizer returns the sprint report computed by the sprint module.
type SprintSummarizer interface {
	SprintSummary(ctx context.Context, actorID uuid.UUID, globalRole string, projectID, sprintID uuid.UUID) (domain.SprintSummary, error)
}

// Result is a report with its generation time and the parameters it used.
type Result struct {
	Report      *domain.Report
	GeneratedAt time.Time
	Params      map[string]any
}

// Service is the use case seen by the HTTP adapter.
type Service interface {
	// List returns the reports available to the actor on the project.
	List(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID) ([]domain.Descriptor, error)
	Run(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID, kind string, raw domain.RawParams) (*Result, error)
}
