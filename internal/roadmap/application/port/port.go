// Package port declares the roadmap in-ports and out-ports.
package port

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/roadmap/domain"
)

// Actions understood by the project authorizer.
const (
	ActionViewProject = "view_project"
	ActionEditIssue   = "edit_issue"
)

// MaxRankedEpics bounds one rank request.
const MaxRankedEpics = 500

// Authorizer decides whether an actor can run an action on a project. The root wires it to the authorization
// module; a project the actor cannot browse answers not found.
type Authorizer interface {
	Authorize(ctx context.Context, action string, actorID uuid.UUID, globalRole string, projectID uuid.UUID) error
}

// Repository reads the raw rows of the roadmap. Epics come ordered by their manual rank, then by creation.
type Repository interface {
	Epics(ctx context.Context, projectID uuid.UUID) ([]domain.EpicRow, error)
	Versions(ctx context.Context, projectID uuid.UUID) ([]domain.PeriodRow, error)
	Sprints(ctx context.Context, projectID uuid.UUID) ([]domain.PeriodRow, error)
	// Dependencies lists the blocks links between epics of the project.
	Dependencies(ctx context.Context, projectID uuid.UUID) ([]domain.Dependency, error)
	// SaveRank stores the manual order of the epics; it fails with a validation error when an id is not an epic of
	// the project.
	SaveRank(ctx context.Context, projectID uuid.UUID, epicIDs []uuid.UUID) error
}

// Roadmap is the answer of the roadmap query.
type Roadmap struct {
	From         *time.Time
	To           *time.Time
	Group        domain.Group
	Bars         []domain.Bar
	Versions     []domain.Bar
	Sprints      []domain.Bar
	Dependencies []domain.Dependency
}

// Service is the roadmap use case.
type Service interface {
	Roadmap(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID, q domain.Query) (Roadmap, error)
	Dependencies(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID) ([]domain.Dependency, error)
	Rank(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID, epicIDs []uuid.UUID) error
}
