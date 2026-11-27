package domain

import "github.com/jhonsferg/corelog/internal/platform/apperror"

var (
	ErrInvalidSummary     = apperror.WithCode(apperror.KindValidation, "issue.summary_required", "summary is required")
	ErrInvalidDescription = apperror.WithCode(apperror.KindValidation, "issue.description_required", "description is required")
	ErrInvalidPriority    = apperror.WithCode(apperror.KindValidation, "issue.invalid_priority", "priority is invalid")
	ErrInvalidStatus      = apperror.WithCode(apperror.KindValidation, "issue.invalid_status", "status is invalid")
	ErrInvalidReporter    = apperror.WithCode(apperror.KindValidation, "issue.reporter_required", "reporter is required")
	ErrInvalidRank        = apperror.WithCode(apperror.KindValidation, "issue.invalid_rank", "issue rank is invalid")
	ErrInvalidIssueType   = apperror.WithCode(apperror.KindValidation, "issue.invalid_type", "issue type is invalid")
	ErrInvalidStoryPoints = apperror.WithCode(apperror.KindValidation, "issue.invalid_story_points", "story points are invalid")
	ErrInvalidSavedFilter = apperror.WithCode(apperror.KindValidation, "issue.invalid_saved_filter", "saved filter is invalid")

	ErrIssueNotFound     = apperror.WithCode(apperror.KindNotFound, "issue.not_found", "issue not found")
	ErrIllegalTransition = apperror.WithCode(apperror.KindConflict, "issue.transition_not_allowed", "illegal issue status transition")
	ErrReporterNotFound  = apperror.WithCode(apperror.KindValidation, "issue.reporter_not_found", "reporter does not exist")
	ErrAssigneeNotFound  = apperror.WithCode(apperror.KindValidation, "issue.assignee_not_found", "assignee does not exist")
	// ErrKeyAmbiguous: the key exists in several organizations the caller can see and no organization_id was given.
	ErrKeyAmbiguous = apperror.WithCode(apperror.KindConflict, "issue.key_ambiguous", "la clave existe en varias organizaciones; indica organization_id")
	// ErrInvalidIssueRef: the path segment is neither a UUID nor a well-formed issue key.
	ErrInvalidIssueRef     = apperror.WithCode(apperror.KindValidation, "issue.invalid_id", "el identificador de la incidencia no es un UUID ni una clave valida")
	ErrCrossTeamAssignment = apperror.WithCode(apperror.KindForbidden, "issue.cross_team_assignment", "the assignee must belong to the same team as the assigner")
)
