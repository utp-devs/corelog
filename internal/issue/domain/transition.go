package domain

import (
	"github.com/google/uuid"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

// StatusRef is the workflow status of an issue as it is read: the organization status id with its name and
// category (todo, in_progress or done).
type StatusRef struct {
	ID       uuid.UUID
	Name     string
	Category string
}

// Status categories of a workflow status.
const (
	StatusCategoryTodo       = "todo"
	StatusCategoryInProgress = "in_progress"
	StatusCategoryDone       = "done"
)

// LegacyStatusOfCategory is the legacy status that mirrors a workflow status category. The legacy column is kept
// in sync for readers that were not moved to workflow statuses yet.
func LegacyStatusOfCategory(category string) Status {
	switch category {
	case StatusCategoryTodo:
		return StatusOpen
	case StatusCategoryInProgress:
		return StatusInProgress
	default:
		return StatusClosed
	}
}

// Typed errors of the transition use case. Messages are Spanish because they reach the user.
var (
	// ErrStatusConflict: the issue changed status after the client read it.
	ErrStatusConflict = apperror.WithCode(apperror.KindConflict, "issue.status_conflict", "el estado de la incidencia cambió; actualiza e inténtalo de nuevo")
	// ErrTransitionUnavailable: no transition leads from the current status to the requested place.
	ErrTransitionUnavailable = apperror.WithCode(apperror.KindConflict, "issue.transition_unavailable", "no hay una transición disponible hacia ese estado")
	// ErrResolutionNotFound: the resolution chosen for the transition does not exist in the organization.
	ErrResolutionNotFound = apperror.WithCode(apperror.KindValidation, "issue.resolution_not_found", "la resolución indicada no existe")
	// ErrTransitionUnsupported: the transition service is not wired.
	ErrTransitionUnsupported = apperror.WithCode(apperror.KindInternal, "issue.transition_unsupported", "las transiciones no están disponibles")
)
