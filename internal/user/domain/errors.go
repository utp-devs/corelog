package domain

import (
	"strconv"

	"github.com/jhonsferg/corelog/internal/platform/apperror"
)

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

// Identity v2 errors: names, phone and phone verification. Codes are stable.
var (
	ErrInvalidPhone  = apperror.WithCode(apperror.KindValidation, "user.invalid_phone", "el telefono no es valido")
	ErrPhoneTaken    = apperror.WithCode(apperror.KindConflict, "user.phone_taken", "este telefono ya esta verificado por otra cuenta")
	ErrInvalidLocale = apperror.WithCode(apperror.KindValidation, "user.locale_invalid", "el idioma no es valido")

	ErrPhoneNotSet          = apperror.WithCode(apperror.KindConflict, "user.phone_not_set", "primero registra un numero de telefono")
	ErrPhoneAlreadyVerified = apperror.WithCode(apperror.KindConflict, "user.phone_already_verified", "el telefono ya esta verificado")
	ErrPhoneSendLimit       = apperror.WithCode(apperror.KindRateLimited, "user.phone_send_limit", "demasiados codigos solicitados: intenta de nuevo en una hora")
	ErrPhoneCodeInvalid     = apperror.WithCode(apperror.KindValidation, "user.phone_code_invalid", "el codigo no es correcto")
	ErrPhoneCodeExpired     = apperror.WithCode(apperror.KindValidation, "user.phone_code_expired", "el codigo vencio o no existe: solicita uno nuevo")
	ErrPhoneCodeAttempts    = apperror.WithCode(apperror.KindRateLimited, "user.phone_code_attempts", "demasiados intentos: solicita un codigo nuevo")
)

// codeInvalidName is the stable code of every first and last name failure.
const codeInvalidName = "user.invalid_name"

func errInvalidNamePart(label string, max int) *apperror.Error {
	return apperror.WithCode(apperror.KindValidation, codeInvalidName,
		"el "+label+" solo admite letras, espacios, guiones y apostrofes (maximo "+strconv.Itoa(max)+" caracteres)")
}

func invalidPhone(msg string) *apperror.Error {
	return apperror.WithCode(apperror.KindValidation, ErrInvalidPhone.Code, msg)
}

// codePasswordMismatch is the code of a password and its confirmation that differ (exact comparison, no
// trimming). Every flow that sets a password (register, change password and any future reset) must reject
// a mismatch before the password policy and before touching storage.
const codePasswordMismatch = "user.password_mismatch"

// PasswordMismatch builds the mismatch error with the confirmation field in its details.
func PasswordMismatch(field string) *apperror.Error {
	const msg = "Las contraseñas no coinciden"
	return apperror.WithCode(apperror.KindValidation, codePasswordMismatch, msg).
		WithDetails(apperror.FieldError{Field: field, Message: msg})
}

// CheckPasswordConfirmation returns PasswordMismatch(field) unless password and confirmation are equal.
func CheckPasswordConfirmation(password, confirmation, field string) error {
	if password != confirmation {
		return PasswordMismatch(field)
	}
	return nil
}
