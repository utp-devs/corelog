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

	// ErrInvitationNotFound is returned for unknown tokens and unknown invitation IDs. It is the same
	// for every unknown token, so lookups never reveal whether an email has an account.
	ErrInvitationNotFound = apperror.WithCode(apperror.KindNotFound, "org.invitation_not_found", "La invitación no existe")
	// ErrInvitationExpired is returned when an invitation is past its expiry.
	ErrInvitationExpired = apperror.WithCode(apperror.KindConflict, "org.invitation_expired", "La invitación ha caducado")
	// ErrInvitationRevoked is returned when an invitation was revoked by an administrator.
	ErrInvitationRevoked = apperror.WithCode(apperror.KindConflict, "org.invitation_revoked", "La invitación fue revocada")
	// ErrInvitationUsed is returned when an invitation was already accepted.
	ErrInvitationUsed = apperror.WithCode(apperror.KindConflict, "org.invitation_used", "La invitación ya fue utilizada")
	// ErrInvitationExists is returned when a pending invitation already exists for the email.
	ErrInvitationExists = apperror.WithCode(apperror.KindConflict, "org.invitation_exists", "Ya existe una invitación pendiente para este correo")
	// ErrAlreadyMember is returned when inviting or accepting for a user who already belongs to the organization.
	ErrAlreadyMember = apperror.WithCode(apperror.KindConflict, "org.already_member", "El usuario ya es miembro de la organización")
	// ErrEmailMismatch is returned when the signed-in account email differs from the invited email.
	ErrEmailMismatch = apperror.WithCode(apperror.KindForbidden, "org.email_mismatch", "La invitación no corresponde a tu correo electrónico")
	// ErrSelfRoleChange is returned when an actor tries to change its own role.
	ErrSelfRoleChange = apperror.WithCode(apperror.KindValidation, "org.self_role_change", "No puedes cambiar tu propio rol")
	// ErrCannotRemoveOwner is returned when a non-owner tries to remove an owner.
	ErrCannotRemoveOwner = apperror.WithCode(apperror.KindForbidden, "org.cannot_remove_owner", "Solo un propietario puede quitar a otro propietario")
	// ErrInvitationEmailInvalid is returned when an invitation email is malformed or too long.
	ErrInvitationEmailInvalid = apperror.WithCode(apperror.KindValidation, "org.invitation_email_invalid", "El correo electrónico de la invitación no es válido")
	// ErrInvitationBatchInvalid is returned when a batch of invitations is empty or exceeds the batch limit.
	ErrInvitationBatchInvalid = apperror.WithCode(apperror.KindValidation, "org.invitation_batch_invalid", "Debes indicar entre 1 y 50 invitaciones")
)
