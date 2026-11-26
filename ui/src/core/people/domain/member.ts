export type OrgRole = 'owner' | 'admin' | 'member' | 'guest'

/** Member of the current organization, as listed by the organization members endpoint. */
export interface OrganizationMember {
  userId: string
  name: string
  email: string
  role: OrgRole
  joinedAt: string
}

/** Client-side allowlist for avatar uploads (must match the API rules). */
export const AVATAR_MAX_BYTES = 2 * 1024 * 1024
export const AVATAR_MIME_TYPES = ['image/png', 'image/jpeg', 'image/webp'] as const
