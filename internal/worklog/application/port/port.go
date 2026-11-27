// Package port declares the in-port of the worklog module and the outbound ports it needs. Issues and
// projects belong to other hexagons, so the module reads and writes their rows only through these narrow
// ports and the SQL adapter in this module.
package port

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/worklog/domain"
)

// Actions of the project authorizer used by this module (names of the permission catalog).
const (
	ActionViewProject   = "view_project"
	ActionLogWork       = "log_work"
	ActionManageWorklog = "manage_worklogs"
	ActionEditIssue     = "edit_issue"
	ActionViewReports   = "view_reports"
)

// Authorizer decides whether an actor may run an action on a project. It is implemented by the root.
// Write actions on archived projects return an archive error.
type Authorizer interface {
	Authorize(ctx context.Context, action string, actorID uuid.UUID, globalRole string, projectID uuid.UUID) error
}

// IssueTime is the time data of one live issue.
type IssueTime struct {
	IssueID        uuid.UUID
	IssueKey       string
	IssueTitle     string
	ProjectID      uuid.UUID
	OrganizationID uuid.UUID
	Original       *int64
	Remaining      *int64
	Spent          int64
}

// Activity is one row of the issue activity history written by this module. The rows use the same shape as
// the issue history so the activity feed renders them without changes.
type Activity struct {
	IssueID  uuid.UUID
	ActorID  uuid.UUID
	Action   string
	Field    string
	OldValue *string
	NewValue *string
	At       time.Time
	BatchID  uuid.UUID
}

// Window selects worklog rows by start instant and optionally by author or project.
type Window struct {
	Lo        time.Time
	Hi        time.Time
	AuthorID  *uuid.UUID
	ProjectID *uuid.UUID
}

// Repository persists worklogs and the time fields of issues. Methods participate in the transaction of
// the context when there is one.
type Repository interface {
	// IssueTime returns the time data of a live issue whose project is not deleted. lock takes a row lock
	// until the end of the transaction. Missing issues return apperror not found.
	IssueTime(ctx context.Context, issueID uuid.UUID, lock bool) (IssueTime, error)
	// Worklog returns a live worklog with its issue and project data. Missing ones return not found.
	Worklog(ctx context.Context, id uuid.UUID, lock bool) (domain.Worklog, error)
	// ListIssueWorklogs returns the live worklogs of an issue, newest start first.
	ListIssueWorklogs(ctx context.Context, issueID uuid.UUID) ([]domain.Worklog, error)
	// InsertWorklog stores a new worklog and fills its id and timestamps.
	InsertWorklog(ctx context.Context, w *domain.Worklog) error
	// UpdateWorklog stores the editable fields and the remaining adjust of a live worklog.
	UpdateWorklog(ctx context.Context, w *domain.Worklog) error
	// SoftDeleteWorklog marks a worklog deleted.
	SoftDeleteWorklog(ctx context.Context, id uuid.UUID, at time.Time) error
	// RecomputeSpent sets issues.time_spent_seconds from the live worklogs of the issue.
	RecomputeSpent(ctx context.Context, issueID uuid.UUID) error
	SetRemaining(ctx context.Context, issueID uuid.UUID, value *int64) error
	SetOriginal(ctx context.Context, issueID uuid.UUID, value *int64) error
	// Window returns the live worklogs whose start is in [Lo, Hi), with issue, project and author data.
	Window(ctx context.Context, w Window) ([]domain.Entry, error)
	// OrgTimes returns the working-time settings of each organization id given. Missing keys mean defaults.
	OrgTimes(ctx context.Context, orgIDs []uuid.UUID) (map[uuid.UUID]domain.OrgTime, error)
	// AddActivities stores the activity rows of one change.
	AddActivities(ctx context.Context, rows []Activity) error
}

// Duration is a duration given as text ("1d 2h") or as seconds. Clear is used by estimates to remove the
// value (JSON null).
type Duration struct {
	Text    *string
	Seconds *int64
	Clear   bool
}

// RemainingInput is the estimate change requested with a worklog. Mode defaults to auto.
type RemainingInput struct {
	Mode  domain.RemainingMode
	Value *Duration
}

// LogInput is a new worklog.
type LogInput struct {
	TimeSpent   Duration
	StartedAt   *time.Time
	Description string
	Remaining   RemainingInput
}

// UpdateInput is a partial change of a worklog. A nil field keeps the current value.
type UpdateInput struct {
	TimeSpent   *Duration
	StartedAt   *time.Time
	Description *string
}

// EstimatesInput changes the original and/or remaining estimate of an issue. A nil field keeps the value.
type EstimatesInput struct {
	Original  *Duration
	Remaining *Duration
}

// Totals are the time totals of one issue.
type Totals struct {
	OriginalSeconds  *int64
	RemainingSeconds *int64
	SpentSeconds     int64
}

// IssueWorklogs is the worklog list of an issue with its totals and the organization working time.
type IssueWorklogs struct {
	Worklogs []domain.Worklog
	Totals   Totals
	Config   domain.Config
}

// Logged is the result of a create, update or estimate change: the worklog (when there is one), the issue
// totals and the working time used to format durations.
type Logged struct {
	Worklog *domain.Worklog
	Totals  Totals
	Config  domain.Config
}

// Estimates is the result of an estimate change.
type Estimates struct {
	Totals Totals
	Config domain.Config
}

// TimesheetQuery selects the worklogs of one user between two dates (YYYY-MM-DD, inclusive, in the timezone
// of each project organization). A nil UserID means the caller.
type TimesheetQuery struct {
	From   string
	To     string
	UserID *uuid.UUID
}

// SummaryQuery selects the worklogs of a project between two dates grouped by user, issue or day.
type SummaryQuery struct {
	From    string
	To      string
	GroupBy string
}

// Service is the in-port used by the HTTP adapter. Every method decides authorization itself.
type Service interface {
	ListIssueWorklogs(ctx context.Context, actorID uuid.UUID, globalRole string, issueID uuid.UUID) (*IssueWorklogs, error)
	LogWork(ctx context.Context, actorID uuid.UUID, globalRole string, issueID uuid.UUID, in LogInput) (*Logged, error)
	UpdateWorklog(ctx context.Context, actorID uuid.UUID, globalRole string, id uuid.UUID, in UpdateInput) (*Logged, error)
	DeleteWorklog(ctx context.Context, actorID uuid.UUID, globalRole string, id uuid.UUID) error
	UpdateEstimates(ctx context.Context, actorID uuid.UUID, globalRole string, issueID uuid.UUID, in EstimatesInput) (*Estimates, error)
	Timesheet(ctx context.Context, actorID uuid.UUID, globalRole string, q TimesheetQuery) (*domain.Timesheet, domain.Config, error)
	TimeSummary(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID, q SummaryQuery) (*domain.Summary, domain.Config, error)
}
