// Package port declares the automation service and the narrow ports the engine needs. The adapters that
// connect issues, search, teams, notifications and webhooks live in the composition root.
package port

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/automation/domain"
)

// Repository stores rules and runs.
type Repository interface {
	Create(ctx context.Context, r *domain.Rule) error
	Get(ctx context.Context, id uuid.UUID) (domain.Rule, error)
	ListByProject(ctx context.Context, projectID uuid.UUID) ([]domain.Rule, error)
	// Update stores the definition when the stored version equals r.Version, then increments it.
	// A stale version returns domain.ErrVersionConflict.
	Update(ctx context.Context, r *domain.Rule) error
	SetEnabled(ctx context.Context, id uuid.UUID, enabled bool, at time.Time) (domain.Rule, error)
	Delete(ctx context.Context, id uuid.UUID) error
	// ProjectOrg returns the organization of a live project.
	ProjectOrg(ctx context.Context, projectID uuid.UUID) (uuid.UUID, bool, error)
	// ActiveFor returns the enabled rules that apply to a project: its own and the organization-wide ones.
	ActiveFor(ctx context.Context, orgID, projectID uuid.UUID) ([]domain.Rule, error)
	// ActiveScheduled returns every enabled scheduled rule.
	ActiveScheduled(ctx context.Context) ([]domain.Rule, error)
	// RecordRun stores a run. Runs that executed (success or failed) also bump run_count and last_run_at.
	RecordRun(ctx context.Context, run *domain.Run) error
	// TouchScheduled sets last_run_at without a run record.
	TouchScheduled(ctx context.Context, id uuid.UUID, at time.Time) error
	// CountRunsSince counts executed runs (success or failed) of a rule since the instant.
	CountRunsSince(ctx context.Context, ruleID uuid.UUID, since time.Time) (int, error)
	ListRuns(ctx context.Context, ruleID uuid.UUID, status string, limit int) ([]domain.Run, error)
}

// ProjectAuthorizer is the project permission check (names of the permission catalog).
type ProjectAuthorizer interface {
	Authorize(ctx context.Context, action string, actorID uuid.UUID, globalRole string, projectID uuid.UUID) error
}

// IssueReader reads the data conditions and smart values need.
type IssueReader interface {
	Snapshot(ctx context.Context, issueID uuid.UUID) (domain.Snapshot, error)
}

// IssueActions performs changes on issues as the automation identity. Every call goes through the issue use
// cases, so permissions, workflow rules, activity and events are the usual ones. The ctx carries the automation
// origin, which the events published by those use cases record.
type IssueActions interface {
	Transition(ctx context.Context, issueID uuid.UUID, statusName string) error
	Assign(ctx context.Context, issueID uuid.UUID, userID uuid.UUID) error
	SetField(ctx context.Context, issueID uuid.UUID, field, value string) error
	AddComment(ctx context.Context, issueID uuid.UUID, body string) error
	AddLabel(ctx context.Context, issueID uuid.UUID, label string) error
	RemoveLabel(ctx context.Context, issueID uuid.UUID, label string) error
	CreateSubtask(ctx context.Context, parentID uuid.UUID, summary string) (string, error)
}

// Matcher evaluates CoreQL text through the search module.
type Matcher interface {
	Matches(ctx context.Context, projectID, issueID uuid.UUID, query string) (bool, error)
	Find(ctx context.Context, projectID uuid.UUID, query string, limit int) ([]uuid.UUID, error)
	Validate(ctx context.Context, query string) error
}

// Directory answers questions about people and teams.
type Directory interface {
	ProjectLead(ctx context.Context, projectID uuid.UUID) (uuid.UUID, error)
	TeamMembers(ctx context.Context, teamID uuid.UUID) ([]uuid.UUID, error)
	// ActorRole returns the project role of a user: lead, contributor, viewer, org_admin or "" when none.
	ActorRole(ctx context.Context, projectID, userID uuid.UUID) (string, error)
	UserName(ctx context.Context, userID uuid.UUID) (string, error)
}

// Notifier sends an in-app notification.
type Notifier interface {
	Notify(ctx context.Context, userID uuid.UUID, kind, message, link string) error
}

// Poster sends the body of a send_webhook action.
type Poster interface {
	Post(ctx context.Context, url string, body []byte) error
}

// RuleInput is the editable definition of a rule.
type RuleInput struct {
	Name       string
	Enabled    *bool
	Trigger    domain.Trigger
	Conditions []domain.Condition
	Actions    []domain.Action
}

// PatchInput changes parts of a rule; nil keeps the value. Version is the version the editor loaded.
type PatchInput struct {
	Name       *string
	Enabled    *bool
	Trigger    *domain.Trigger
	Conditions *[]domain.Condition
	Actions    *[]domain.Action
	Version    *int
}

// Validation is the dry-run result: whether the rule is valid and what its actions would do for a sample issue.
type Validation struct {
	Valid   bool
	Errors  []string
	Planned []string
}

// Service is the in-port.
type Service interface {
	Create(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID, in RuleInput) (domain.Rule, error)
	List(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID) ([]domain.Rule, error)
	Get(ctx context.Context, actorID uuid.UUID, globalRole string, id uuid.UUID) (domain.Rule, error)
	Patch(ctx context.Context, actorID uuid.UUID, globalRole string, id uuid.UUID, in PatchInput) (domain.Rule, error)
	Delete(ctx context.Context, actorID uuid.UUID, globalRole string, id uuid.UUID) error
	SetEnabled(ctx context.Context, actorID uuid.UUID, globalRole string, id uuid.UUID, enabled bool) (domain.Rule, error)
	Runs(ctx context.Context, actorID uuid.UUID, globalRole string, id uuid.UUID, status string, limit int) ([]domain.Run, error)
	// Validate checks a definition and, with a sample issue, lists what the actions would do without running them.
	Validate(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID, in RuleInput, sampleIssue *uuid.UUID) (Validation, error)
}
