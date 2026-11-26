export type OrgRole = 'owner' | 'admin' | 'member' | 'guest'

/** Member of the current organization, as listed by the organization members endpoint. */
export interface OrganizationMember {
  userId: string
  name: string
  displayName: string
  email: string
  jobTitle: string
  department: string
  /** When false the avatar is not requested and initials are shown. */
  hasAvatar: boolean
  role: OrgRole
  joinedAt: string
}

/** Public profile of one member (GET /organizations/{org}/people/{user_id}). Timezone is part of it. */
export interface PersonProfile {
  userId: string
  name: string
  displayName: string
  email: string
  jobTitle: string
  department: string
  timezone: string
  hasAvatar: boolean
  orgRole: OrgRole
  joinedAt: string
}

/** Client-side allowlist for avatar uploads (must match the API rules). */
export const AVATAR_MAX_BYTES = 2 * 1024 * 1024
export const AVATAR_MIME_TYPES = ['image/png', 'image/jpeg', 'image/webp'] as const
