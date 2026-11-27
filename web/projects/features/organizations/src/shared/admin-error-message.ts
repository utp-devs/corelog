import { ApiError, isApiError } from '@corelog/core';

const MESSAGES: Readonly<Record<string, string>> = {
  'org.last_owner':
    'Es el último propietario de la organización. Transfiere la propiedad a otro miembro antes de continuar.',
  'org.forbidden': 'No tienes permiso para realizar esta acción.',
  'org.self_role_change': 'No puedes cambiar tu propio rol.',
  'org.cannot_remove_owner': 'Solo un propietario puede quitar a otro propietario.',
  'org.member_not_found': 'Esa persona ya no es miembro de la organización.',
  'org.role_invalid': 'El rol elegido no es válido.',
  'org.transfer_self': 'No puedes transferirte la propiedad a ti mismo.',
  'org.confirmation_mismatch':
    'El texto escrito no coincide con el identificador de la organización.',
  'org.already_member': 'Esa persona ya es miembro de la organización.',
  'org.invitation_exists': 'Ya hay una invitación pendiente para ese correo.',
  'org.invitation_not_found': 'La invitación ya no existe.',
  'org.invitation_used': 'La invitación ya fue aceptada.',
  'org.invitation_revoked': 'La invitación ya fue revocada.',
  'org.logo_too_large': 'La imagen supera el tamaño máximo de 2 MB.',
  'org.logo_invalid': 'El archivo no es una imagen PNG, JPEG o WebP válida.',
};

/** Spanish message for an organization admin failure, keyed by the API code. Falls back to the server text. */
export function adminErrorMessage(error: unknown, fallback: string): string {
  if (!isApiError(error)) return fallback;
  const known = MESSAGES[error.code];
  if (known) return known;
  return error.message || fallback;
}

/** True when the failure is the last-owner protection, so pages can explain it inline. */
export function isLastOwnerError(error: unknown): boolean {
  return isApiError(error) && (error as ApiError).code === 'org.last_owner';
}
