package domain

import "github.com/jhonsferg/corelog/internal/platform/apperror"

var (
	ErrInvalidName     = apperror.WithCode(apperror.KindValidation, "user.name_required", "name is required")
	ErrInvalidEmail    = apperror.WithCode(apperror.KindValidation, "user.email_invalid", "a valid email is required")
	ErrInvalidPassword = apperror.WithCode(apperror.KindValidation, "user.password_required", "password is required")
	ErrInvalidRole     = apperror.WithCode(apperror.KindValidation, "user.role_invalid", "role is invalid")

	ErrUserNotFound       = apperror.WithCode(apperror.KindNotFound, "user.not_found", "user not found")
	ErrEmailAlreadyTaken  = apperror.WithCode(apperror.KindConflict, "auth.email_taken", "email is already registered")
	ErrInvalidCredentials = apperror.WithCode(apperror.KindUnauthorized, "auth.invalid_credentials", "invalid email or password")
	ErrIncorrectPassword  = apperror.WithCode(apperror.KindUnauthorized, "auth.incorrect_password", "current password is incorrect")

	ErrSessionNotFound = apperror.WithCode(apperror.KindNotFound, "auth.session_not_found", "session not found")
)
