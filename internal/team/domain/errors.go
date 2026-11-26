package domain

import "github.com/jhonsferg/corelog/internal/platform/apperror"

var (
	ErrInvalidName   = apperror.WithCode(apperror.KindValidation, "team.name_required", "name is required")
	ErrTeamNotFound  = apperror.WithCode(apperror.KindNotFound, "team.not_found", "team not found")
	ErrTeamNameTaken = apperror.WithCode(apperror.KindConflict, "team.name_taken", "a team with this name already exists")
)
