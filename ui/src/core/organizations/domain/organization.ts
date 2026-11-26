// Organization context types. Mirrors the schemas of openapi.yaml (Organization, OrganizationMember,
// Invitation, InvitationPreview, OrganizationMembership). Camel case here; snake_case lives in the adapter.

export const ORG_ROLES = ['owner', 'admin', 'member', 'guest'] as const
export type OrgRole = (typeof ORG_ROLES)[number]

export type OrgWhoCan = 'members' | 'admins'
export type OrgDefaultVisibility = 'open' | 'private'

export interface OrganizationSettings {
  whoCanCreateProjects: OrgWhoCan
  whoCanCreateTeams: OrgWhoCan
  defaultProjectVisibility: OrgDefaultVisibility
  require2fa: boolean
  maxUploadBytes: number
  hoursPerDay: number
  daysPerWeek: number
  timezone: string
}

export interface Organization {
  id: string
  slug: string
  name: string
  description: string
  settings: OrganizationSettings
  /** Role of the caller. Present on list and detail responses. */
  role?: OrgRole
  createdAt: string
  updatedAt: string
}

export interface OrganizationMember {
  userId: string
  name: string
  email: string
  role: OrgRole
  joinedAt: string
}

export type InvitationStatus = 'pending' | 'accepted' | 'revoked' | 'expired'

export interface Invitation {
  id: string
  email: string
  role: OrgRole
  status: InvitationStatus
  invitedBy: string | null
  expiresAt: string
  acceptedAt: string | null
  revokedAt: string | null
  createdAt: string
}

/** Invitation returned right after creation or resend: the only time the plain token is available. */
export interface InvitationCreated extends Invitation {
  token: string
  acceptUrl: string
}

/** Public preview of an invitation behind a token. */
export interface InvitationPreview {
  organizationName: string
  role: OrgRole
  email: string
  status: InvitationStatus
  expiresAt: string
}

export interface OrganizationMembership {
  organizationId: string
  userId: string
  role: OrgRole
  joinedAt: string
}

/** Owners and admins manage the organization (settings, members, invitations). */
export function canManageOrg(role: OrgRole | null | undefined): boolean {
  return role === 'owner' || role === 'admin'
}

/** Slug rule of the API: lowercase letters, digits and hyphens, 3 to 40 characters, no edge hyphen. */
export const ORG_SLUG_PATTERN = /^[a-z0-9][a-z0-9-]{1,38}[a-z0-9]$/

export function isValidOrgSlug(slug: string): boolean {
  return ORG_SLUG_PATTERN.test(slug)
}

/** Derives a slug from a name: strips accents, lowercases, joins words with hyphens, max 40 characters. */
export function slugFromName(name: string): string {
  return name
    .normalize('NFD')
    .replace(/[̀-ͯ]/g, '')
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, 40)
    .replace(/-+$/g, '')
}

const TOKEN_PATTERN = /^[A-Za-z0-9_-]{16,}$/

/**
 * Extracts the invitation token from a pasted accept link (".../invite/<token>") or from a bare token.
 * Returns null when the input has neither shape.
 */
export function parseInvitationToken(input: string): string | null {
  const value = input.trim()
  const fromLink = /\/invite\/([A-Za-z0-9_-]+)\/?(?:[?#].*)?$/.exec(value)
  if (fromLink) return fromLink[1]
  return TOKEN_PATTERN.test(value) ? value : null
}
