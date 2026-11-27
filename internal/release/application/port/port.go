// Package port declares the in-ports (service) and out-ports (repository, authorizer, narrow collaborators) of
// the release module.
package port

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/events"
	"github.com/jhonsferg/corelog/internal/release/domain"
)

// Action names understood by the project authorizer.
const (
	ActionViewProject    = "view_project"
	ActionManageVersions = "manage_versions"
)

// Authorizer decides whether an actor can run an action on a project. The root wires it to the authorization module.
type Authorizer interface {
	Authorize(ctx context.Context, action string, actorID uuid.UUID, globalRole string, projectID uuid.UUID) error
}

// ProjectKeys resolves a project key (for example CORE) to its id through the project visibility rules.
type ProjectKeys interface {
	ResolveProjectKey(ctx context.Context, actorID uuid.UUID, globalRole string, orgID uuid.UUID, key string) (uuid.UUID, error)
}

// EventPublisher records domain events in the transaction of the change.
type EventPublisher interface {
	Publish(ctx context.Context, e events.Event) error
}

// TxRunner runs fn in one transaction. database.TxManager satisfies it.
type TxRunner interface {
	InTx(ctx context.Context, fn func(ctx context.Context) error) error
}

// Repository is the persistence of versions and their issue links.
type Repository interface {
	Create(ctx context.Context, v *domain.Version) error
	Save(ctx context.Context, v *domain.Version) error
	FindByID(ctx context.Context, id uuid.UUID) (*domain.Version, error)
	// LockByID reads a version and locks its row until the transaction ends.
	LockByID(ctx context.Context, id uuid.UUID) (*domain.Version, error)
	List(ctx context.Context, projectID uuid.UUID, status *domain.Status) ([]*domain.Version, error)
	// Progress returns the fix-issue progress of each version; versions without issues are absent.
	Progress(ctx context.Context, versionIDs []uuid.UUID) (map[uuid.UUID][]domain.CategoryCount, error)
	NameTaken(ctx context.Context, projectID uuid.UUID, name string, except uuid.UUID) (bool, error)
	Delete(ctx context.Context, id uuid.UUID) error
	// MoveLinks moves every fix and affects link of from to to, skipping issues already linked to to.
	MoveLinks(ctx context.Context, from, to uuid.UUID) error
	// MoveUnresolvedFixes moves the fix links of issues that are not done and returns how many moved.
	MoveUnresolvedFixes(ctx context.Context, from, to uuid.UUID) (int, error)
	// UnresolvedFixCount counts the live fix issues of a version that are not done.
	UnresolvedFixCount(ctx context.Context, id uuid.UUID) (int, error)
	NotesIssues(ctx context.Context, id uuid.UUID) ([]domain.NoteIssue, error)
}

// ListFilter narrows the version list. A nil Status lists every version.
type ListFilter struct {
	Status *domain.Status
}

// CreateInput is the body of a new version.
type CreateInput struct {
	Name        string
	Description string
	StartDate   *time.Time
	ReleaseDate *time.Time
}

// MoveTarget says where the unresolved issues of a released version go. Mode is "none", "version" or "new".
type MoveTarget struct {
	Mode      string
	VersionID uuid.UUID
}

// ReleaseInput is the body of the release action.
type ReleaseInput struct {
	ReleaseDate *time.Time
	Move        MoveTarget
}

// ReleaseResult summarises a release: how many unresolved issues moved, how many stayed, and the new version
// when one was created.
type ReleaseResult struct {
	Version          *domain.Version
	MovedIssues      int
	UnresolvedIssues int
	NextVersion      *domain.Version
}

// Service is the use case of versions and release notes. Permissions are decided here.
type Service interface {
	List(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID, filter ListFilter) ([]*domain.Version, error)
	Create(ctx context.Context, actorID uuid.UUID, globalRole string, projectID uuid.UUID, in CreateInput) (*domain.Version, error)
	Get(ctx context.Context, actorID uuid.UUID, globalRole string, id uuid.UUID) (*domain.Version, error)
	Update(ctx context.Context, actorID uuid.UUID, globalRole string, id uuid.UUID, patch domain.VersionPatch) (*domain.Version, error)
	// Delete removes the version. moveTo, when set, takes over its fix and affects links.
	Delete(ctx context.Context, actorID uuid.UUID, globalRole string, id uuid.UUID, moveTo *uuid.UUID) error
	Release(ctx context.Context, actorID uuid.UUID, globalRole string, id uuid.UUID, in ReleaseInput) (*ReleaseResult, error)
	Archive(ctx context.Context, actorID uuid.UUID, globalRole string, id uuid.UUID) (*domain.Version, error)
	Notes(ctx context.Context, actorID uuid.UUID, globalRole string, id uuid.UUID) (*domain.ReleaseNotes, error)
}
