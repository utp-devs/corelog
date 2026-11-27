import { OrgRole } from '../models/organization.model';
import { asApiError } from './api-error';

export const ROLE_LABELS: Readonly<Record<OrgRole, string>> = {
  owner: 'Propietario',
  admin: 'Administrador',
  member: 'Miembro',
  guest: 'Invitado',
};

/** Role choices for invitations. Owners are granted by transfer, not by invitation. */
export const INVITABLE_ROLE_OPTIONS: readonly {
  readonly value: OrgRole;
  readonly label: string;
}[] = (['admin', 'member', 'guest'] as const).map((role) => ({
  value: role,
  label: ROLE_LABELS[role],
}));

export const ROLE_DESCRIPTIONS: Readonly<Record<OrgRole, string>> = {
  owner: 'Control total de la organización.',
  admin: 'Gestiona miembros, invitaciones y configuración.',
  member: 'Trabaja en los proyectos y equipos a los que se le asigna.',
  guest: 'Acceso limitado a lo que se le comparte.',
};

/** Spanish message for an invitation-acceptance failure, keyed by the API code. */
export function invitationErrorMessage(error: unknown, invitedEmail?: string): string {
  const apiError = asApiError(error);
  switch (apiError.code) {
    case 'org.email_mismatch':
      return invitedEmail
        ? `Esta invitación es para ${invitedEmail}. Inicia sesión con esa cuenta para aceptarla.`
        : 'Esta invitación es para otra cuenta. Inicia sesión con la cuenta invitada.';
    case 'org.invitation_used':
      return 'Esta invitación ya se ha utilizado.';
    case 'org.invitation_revoked':
      return 'Esta invitación fue revocada. Pide a un administrador que te envíe una nueva.';
    case 'org.invitation_expired':
      return 'La invitación ha caducado. Pide a un administrador un enlace nuevo.';
    case 'org.already_member':
      return 'Ya eres miembro de esta organización.';
    case 'org.invitation_not_found':
      return 'Este enlace no es válido.';
    default:
      return apiError.code === 'unknown'
        ? 'No se pudo completar la operación. Inténtalo de nuevo.'
        : apiError.message;
  }
}

/** Spanish message for an organization create or slug failure. Field errors are mapped separately. */
export function orgCreateErrorMessage(error: unknown): string {
  const apiError = asApiError(error);
  switch (apiError.code) {
    case 'org.limit_reached':
      return 'Has alcanzado el máximo de organizaciones que puedes crear (5).';
    case 'org.slug_taken':
      return 'Ese identificador ya está en uso. Elige otro.';
    case 'org.slug_reserved':
      return 'Ese identificador está reservado. Elige otro.';
    default:
      return apiError.code === 'unknown'
        ? 'No se pudo crear la organización. Inténtalo de nuevo.'
        : apiError.message;
  }
}
