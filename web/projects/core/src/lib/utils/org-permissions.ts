import { OrgRole, OrgWhoCan } from '../models/organization.model';

/** Upper bound of the organization logo, as enforced by the API (PUT /organizations/{id}/logo). */
export const LOGO_MAX_BYTES = 2 * 1024 * 1024;
export const LOGO_MIME_TYPES: readonly string[] = ['image/png', 'image/jpeg', 'image/webp'];

/** Owners and admins manage the organization (settings, members, invitations). */
export function canManageOrg(role: OrgRole | null | undefined): boolean {
  return role === 'owner' || role === 'admin';
}

/** Spanish message for a logo file that the API would reject, or null when the file is acceptable. */
export function validateLogoFile(file: {
  readonly type: string;
  readonly size: number;
}): string | null {
  if (!LOGO_MIME_TYPES.includes(file.type)) {
    return 'Formato no admitido. Usa una imagen PNG, JPEG o WebP.';
  }
  if (file.size > LOGO_MAX_BYTES) {
    return 'La imagen supera el tamaño máximo de 2 MB.';
  }
  return null;
}

/** Settings, logo and invitations: owners and admins. */
export function canEditOrgSettings(role: OrgRole | null | undefined): boolean {
  return canManageOrg(role);
}

/** Ownership transfer and organization deletion: owners only. */
export function canUseOwnerDangerZone(role: OrgRole | null | undefined): boolean {
  return role === 'owner';
}

/** Any member (not a guest) may read the member list. */
export function canViewMembers(role: OrgRole | null | undefined): boolean {
  return role === 'owner' || role === 'admin' || role === 'member';
}

/** Roles the actor may assign to the target. Never to self (the API answers org.self_role_change). */
export function assignableRoles(
  actor: OrgRole | null | undefined,
  target: OrgRole,
  isSelf: boolean,
): OrgRole[] {
  if (isSelf) {
    return [];
  }
  if (actor === 'owner') {
    return ['owner', 'admin', 'member', 'guest'];
  }
  if (actor === 'admin' && (target === 'member' || target === 'guest')) {
    return ['member', 'guest'];
  }
  return [];
}

/** Owners may remove anyone else; admins may remove members and guests only. */
export function canRemoveMember(
  actor: OrgRole | null | undefined,
  target: OrgRole,
  isSelf: boolean,
): boolean {
  if (isSelf) {
    return false;
  }
  if (actor === 'owner') {
    return true;
  }
  return actor === 'admin' && (target === 'member' || target === 'guest');
}

/** Roles an actor may grant through an invitation. Owners are granted by transfer only. */
export function invitableRoles(actor: OrgRole | null | undefined): OrgRole[] {
  if (actor === 'owner') {
    return ['admin', 'member', 'guest'];
  }
  if (actor === 'admin') {
    return ['member', 'guest'];
  }
  return [];
}

export interface CreateProjectRights {
  readonly allowed: boolean;
  /** Spanish explanation shown when creation is not allowed. */
  readonly reason: string | null;
}

/**
 * Who may create projects: global admins, organization owners and admins, and organization members
 * when the organization policy opens creation to members.
 */
export function createProjectRights(
  globalRole: string | null | undefined,
  orgRole: OrgRole | null | undefined,
  policy: OrgWhoCan | null | undefined,
): CreateProjectRights {
  if (globalRole === 'admin' || orgRole === 'owner' || orgRole === 'admin') {
    return { allowed: true, reason: null };
  }
  if (orgRole === 'member' && policy === 'members') return { allowed: true, reason: null };
  if (orgRole === 'guest') {
    return { allowed: false, reason: 'Los invitados no pueden crear proyectos.' };
  }
  return {
    allowed: false,
    reason: 'Solo los administradores de la organización pueden crear proyectos.',
  };
}
