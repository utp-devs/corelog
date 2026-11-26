import type { OrgRole } from '@/core/organizations/domain/organization'
import { canManageOrg } from '@/core/organizations/domain/organization'

/** Upper bound of the organization logo, as enforced by the API (PUT /organizations/{org}/logo). */
export const LOGO_MAX_BYTES = 2 * 1024 * 1024
export const LOGO_MIME_TYPES = ['image/png', 'image/jpeg', 'image/webp'] as const

/** Spanish message for a logo file that the API would reject, or null when the file is acceptable. */
export function validateLogoFile(file: { type: string; size: number }): string | null {
  if (!(LOGO_MIME_TYPES as readonly string[]).includes(file.type)) {
    return 'Formato no admitido. Usa una imagen PNG, JPEG o WebP.'
  }
  if (file.size > LOGO_MAX_BYTES) {
    return 'La imagen supera el tamaño máximo de 2 MB.'
  }
  return null
}

/** Settings, logo and invitations: owners and admins. */
export function canEditOrgSettings(role: OrgRole | null | undefined): boolean {
  return canManageOrg(role)
}

/** Ownership transfer and organization deletion: owners only. */
export function canUseOwnerDangerZone(role: OrgRole | null | undefined): boolean {
  return role === 'owner'
}

/** Any member (not a guest) may read the member list. */
export function canViewMembers(role: OrgRole | null | undefined): boolean {
  return role === 'owner' || role === 'admin' || role === 'member'
}

/** Roles the actor may assign to the target. Never to self (the API answers org.self_role_change). */
export function assignableRoles(actor: OrgRole | null | undefined, target: OrgRole, isSelf: boolean): OrgRole[] {
  if (isSelf) return []
  if (actor === 'owner') return ['owner', 'admin', 'member', 'guest']
  if (actor === 'admin' && (target === 'member' || target === 'guest')) return ['member', 'guest']
  return []
}

/** Owners may remove anyone else; admins may remove members and guests only. */
export function canRemoveMember(actor: OrgRole | null | undefined, target: OrgRole, isSelf: boolean): boolean {
  if (isSelf) return false
  if (actor === 'owner') return true
  return actor === 'admin' && (target === 'member' || target === 'guest')
}

/** Roles an actor may grant through an invitation. Owners are granted by transfer only. */
export function invitableRoles(actor: OrgRole | null | undefined): OrgRole[] {
  if (actor === 'owner') return ['admin', 'member', 'guest']
  if (actor === 'admin') return ['member', 'guest']
  return []
}
