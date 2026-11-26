package domain

import "github.com/jhonsferg/corelog/internal/platform/apperror"

var (
	ErrInvalidTitle       = apperror.WithCode(apperror.KindValidation, "issue.title_required", "title is required")
	ErrInvalidDescription = apperror.WithCode(apperror.KindValidation, "issue.description_required", "description is required")
	ErrInvalidPriority    = apperror.WithCode(apperror.KindValidation, "issue.invalid_priority", "priority is invalid")
	ErrInvalidStatus      = apperror.WithCode(apperror.KindValidation, "issue.invalid_status", "status is invalid")
	ErrInvalidRequester   = apperror.WithCode(apperror.KindValidation, "issue.requester_required", "requester is required")
	ErrInvalidRank        = apperror.WithCode(apperror.KindValidation, "issue.invalid_rank", "issue rank is invalid")
	ErrInvalidIssueType   = apperror.WithCode(apperror.KindValidation, "issue.invalid_type", "issue type is invalid")
	ErrInvalidStoryPoints = apperror.WithCode(apperror.KindValidation, "issue.invalid_story_points", "story points are invalid")
	ErrInvalidSavedFilter = apperror.WithCode(apperror.KindValidation, "issue.invalid_saved_filter", "saved filter is invalid")

	ErrTicketNotFound      = apperror.WithCode(apperror.KindNotFound, "issue.not_found", "ticket not found")
	ErrIllegalTransition   = apperror.WithCode(apperror.KindConflict, "issue.transition_not_allowed", "illegal ticket status transition")
	ErrRequesterNotFound   = apperror.WithCode(apperror.KindValidation, "issue.requester_not_found", "requester does not exist")
	ErrAssigneeNotFound    = apperror.WithCode(apperror.KindValidation, "issue.assignee_not_found", "assignee does not exist")
	ErrCrossTeamAssignment = apperror.WithCode(apperror.KindForbidden, "issue.cross_team_assignment", "the assignee must belong to the same team as the assigner")
)
