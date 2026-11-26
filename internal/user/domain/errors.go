package domain

import "github.com/jhonsferg/corelog/internal/platform/apperror"

var (
	ErrInvalidName        = apperror.WithCode(apperror.KindValidation, "user.name_required", "name is required")
	ErrInvalidDisplayName = apperror.WithCode(apperror.KindValidation, "user.display_name_required", "display name is required")
	ErrInvalidEmail       = apperror.WithCode(apperror.KindValidation, "user.email_invalid", "a valid email is required")
	ErrInvalidPassword    = apperror.WithCode(apperror.KindValidation, "user.password_required", "password is required")
	ErrInvalidRole        = apperror.WithCode(apperror.KindValidation, "user.role_invalid", "role is invalid")
	ErrInvalidTimezone    = apperror.WithCode(apperror.KindValidation, "user.timezone_invalid", "la zona horaria no es valida")

	ErrUserNotFound       = apperror.WithCode(apperror.KindNotFound, "user.not_found", "user not found")
	ErrEmailAlreadyTaken  = apperror.WithCode(apperror.KindConflict, "auth.email_taken", "email is already registered")
	ErrInvalidCredentials = apperror.WithCode(apperror.KindUnauthorized, "auth.invalid_credentials", "invalid email or password")
	ErrIncorrectPassword  = apperror.WithCode(apperror.KindUnauthorized, "auth.incorrect_password", "current password is incorrect")

	// ErrNotOrganizationMember is returned when the caller is not a member of the organization.
	// It is a 404 so that the existence of the organization is not revealed.
	ErrNotOrganizationMember = apperror.WithCode(apperror.KindNotFound, "organization.not_found", "organizacion no encontrada")

	ErrSessionNotFound = apperror.WithCode(apperror.KindNotFound, "auth.session_not_found", "session not found")

	// ErrSoleOwner blocks closing an account that is the only owner of a live organization.
	ErrSoleOwner = apperror.WithCode(apperror.KindConflict, "user.sole_owner", "Eres el único propietario de una organización: transfiere la propiedad o elimínala antes de cerrar tu cuenta")
)
