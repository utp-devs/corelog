package domain

import "github.com/jhonsferg/corelog/internal/platform/apperror"

var (
	ErrOrganizationRequired = apperror.WithCode(apperror.KindValidation, "team.organization_required", "organization_id es obligatorio")
	ErrOrganizationNotFound = apperror.WithCode(apperror.KindNotFound, "team.organization_not_found", "organización no encontrada")
	ErrMemberIDInvalid      = apperror.WithCode(apperror.KindValidation, "team.member_id_invalid", "member_id debe ser un UUID válido")
	ErrInvalidName          = apperror.WithCode(apperror.KindValidation, "team.name_required", "el nombre es obligatorio y debe tener entre 1 y 120 caracteres")
	ErrInvalidDescription   = apperror.WithCode(apperror.KindValidation, "team.description_too_long", "la descripción no puede superar 500 caracteres")
	ErrInvalidAvatarColor   = apperror.WithCode(apperror.KindValidation, "team.avatar_color_invalid", "el color debe tener formato #RRGGBB")
	ErrInvalidRole          = apperror.WithCode(apperror.KindValidation, "team.role_invalid", "el rol del equipo debe ser maintainer o member")
	ErrUserNotInOrg         = apperror.WithCode(apperror.KindValidation, "team.user_not_in_org", "el usuario no pertenece a la organización")
	ErrTeamNotFound         = apperror.WithCode(apperror.KindNotFound, "team.not_found", "equipo no encontrado")
	ErrTeamNameTaken        = apperror.WithCode(apperror.KindConflict, "team.name_taken", "ya existe un equipo con este nombre")
	ErrMemberNotFound       = apperror.WithCode(apperror.KindNotFound, "team.member_not_found", "miembro del equipo no encontrado")
	ErrMemberExists         = apperror.WithCode(apperror.KindConflict, "team.member_exists", "el usuario ya es miembro de este equipo")
	ErrForbidden            = apperror.WithCode(apperror.KindForbidden, "team.forbidden", "no tienes permisos para realizar esta acción en el equipo")
	ErrCreationNotAllowed   = apperror.WithCode(apperror.KindForbidden, "team.creation_not_allowed", "no tienes permisos para crear equipos en esta organización")
)
