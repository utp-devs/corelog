package domain

import "github.com/jhonsferg/corelog/internal/platform/apperror"

// Typed errors of the organization hexagon. Codes live in the org.* namespace.
// Shared values must not be mutated; use apperror.WithCode to build variants.
var (
	// ErrNotFound is returned for organizations that do not exist, are deleted, or are not visible to the actor.
	ErrNotFound = apperror.WithCode(apperror.KindNotFound, "org.not_found", "Organización no encontrada")
	// ErrMemberNotFound is returned when the user is not a member of the organization.
	ErrMemberNotFound = apperror.WithCode(apperror.KindNotFound, "org.member_not_found", "El miembro no existe en la organización")
	// ErrMemberExists is returned when adding a user who is already a member.
	ErrMemberExists = apperror.WithCode(apperror.KindConflict, "org.member_exists", "El usuario ya es miembro de la organización")
	// ErrSlugTaken is returned when another organization already uses the slug (deleted ones included).
	ErrSlugTaken = apperror.WithCode(apperror.KindConflict, "org.slug_taken", "El identificador ya está en uso")
	// ErrSlugInvalid is returned when the slug does not match the slug pattern.
	ErrSlugInvalid = apperror.WithCode(apperror.KindValidation, "org.slug_invalid", "El identificador debe tener entre 3 y 40 caracteres: minúsculas, números y guiones, sin guion al inicio ni al final")
	// ErrSlugReserved is returned for slugs reserved by the platform.
	ErrSlugReserved = apperror.WithCode(apperror.KindValidation, "org.slug_reserved", "Este identificador está reservado")
	// ErrLastOwner is returned when an operation would leave the organization without owners.
	ErrLastOwner = apperror.WithCode(apperror.KindConflict, "org.last_owner", "La organización debe tener al menos un propietario")
	// ErrForbidden is returned when the actor can see the organization but may not perform the action.
	ErrForbidden = apperror.WithCode(apperror.KindForbidden, "org.forbidden", "No tienes permiso para realizar esta acción")
	// ErrNameRequired is returned when the organization name is empty after trimming.
	ErrNameRequired = apperror.WithCode(apperror.KindValidation, "org.name_required", "El nombre de la organización es obligatorio")
	// ErrSettingsInvalid is the code and message used by Settings.Validate; the returned error carries field details.
	ErrSettingsInvalid = apperror.WithCode(apperror.KindValidation, "org.settings_invalid", "La configuración de la organización no es válida")
	// ErrConfirmationMismatch is returned when the typed slug does not match the organization slug.
	ErrConfirmationMismatch = apperror.WithCode(apperror.KindValidation, "org.confirmation_mismatch", "El identificador escrito no coincide con el de la organización")
	// ErrRoleInvalid is returned for unknown organization roles.
	ErrRoleInvalid = apperror.WithCode(apperror.KindValidation, "org.role_invalid", "El rol indicado no es válido")
	// ErrTransferSelf is returned when ownership is transferred to the actor.
	ErrTransferSelf = apperror.WithCode(apperror.KindValidation, "org.transfer_self", "No puedes transferir la propiedad a ti mismo")
)
