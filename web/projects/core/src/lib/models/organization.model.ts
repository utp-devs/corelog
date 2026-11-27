/** Organization context types. Mirrors the schemas of the CoreLog API (Organization, OrganizationMember, Invitation). */

export const ORG_ROLES = ['owner', 'admin', 'member', 'guest'] as const;
export type OrgRole = (typeof ORG_ROLES)[number];

export type OrgWhoCan = 'members' | 'admins';
export type OrgDefaultVisibility = 'open' | 'private';

export interface OrganizationSettings {
  readonly whoCanCreateProjects: OrgWhoCan;
  readonly whoCanCreateTeams: OrgWhoCan;
  readonly defaultProjectVisibility: OrgDefaultVisibility;
  /** Read-only in the interface: the API owns the second factor policy. */
  readonly require2fa: boolean;
  readonly maxUploadBytes: number;
  readonly hoursPerDay: number;
  readonly daysPerWeek: number;
  readonly timezone: string;
}

export interface Organization {
  readonly id: string;
  readonly slug: string;
  readonly name: string;
  readonly description: string;
  readonly settings: OrganizationSettings;
  /** Role of the caller. Present on list and detail responses. */
  readonly role?: OrgRole;
  /** True when the organization has a logo (GET /organizations/{id}/logo answers 200). */
  readonly hasLogo: boolean;
  readonly createdAt: string;
  readonly updatedAt: string;
}

export interface OrganizationMember {
  readonly userId: string;
  readonly name: string;
  readonly displayName: string;
  readonly email: string;
  readonly jobTitle: string;
  readonly department: string;
  /** When false the avatar is not requested and initials are shown. */
  readonly hasAvatar: boolean;
  readonly role: OrgRole;
  readonly joinedAt: string;
}

export const INVITATION_STATUSES = ['pending', 'accepted', 'revoked', 'expired'] as const;
export type InvitationStatus = (typeof INVITATION_STATUSES)[number];

/** Which invitations a list request returns. */
export type InvitationScope = 'pending' | 'all';

export interface Invitation {
  readonly id: string;
  readonly email: string;
  readonly role: OrgRole;
  readonly status: InvitationStatus;
  readonly invitedBy: string | null;
  readonly expiresAt: string;
  readonly acceptedAt: string | null;
  readonly revokedAt: string | null;
  readonly createdAt: string;
}

/** Invitation returned right after creation or resend: the only time the plain token is available. */
export interface InvitationCreated extends Invitation {
  readonly token: string;
  readonly acceptUrl: string;
}

/** Public preview of an invitation behind a token. */
export interface InvitationPreview {
  readonly organizationName: string;
  readonly role: OrgRole;
  readonly email: string;
  readonly status: InvitationStatus;
  readonly expiresAt: string;
}

export interface OrganizationMembership {
  readonly organizationId: string;
  readonly userId: string;
  readonly role: OrgRole;
  readonly joinedAt: string;
}

export interface CreateOrganizationInput {
  readonly name: string;
  readonly slug?: string;
  readonly description?: string;
}

export interface InvitationItemInput {
  readonly email: string;
  readonly role: OrgRole;
}

/** Partial settings update: omitted fields keep their value. */
export type OrgSettingsPatch = Partial<OrganizationSettings>;

/** Partial organization update (PATCH /organizations/{id}). */
export interface OrganizationPatch {
  readonly name?: string;
  readonly description?: string;
  readonly settings?: OrgSettingsPatch;
}

/** Slug rule of the API: lowercase letters, digits and hyphens, 3 to 40 characters, no edge hyphen. */
export const ORG_SLUG_PATTERN = /^[a-z0-9][a-z0-9-]{1,38}[a-z0-9]$/;

export function isValidOrgSlug(slug: string): boolean {
  return ORG_SLUG_PATTERN.test(slug);
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
    .replace(/-+$/g, '');
}

const TOKEN_PATTERN = /^[A-Za-z0-9_-]{16,}$/;

/**
 * Extracts the invitation token from a pasted accept link (".../invite/<token>") or from a bare token.
 * Returns null when the input has neither shape.
 */
export function parseInvitationToken(input: string): string | null {
  const value = input.trim();
  const fromLink = /\/invite\/([A-Za-z0-9_-]+)\/?(?:[?#].*)?$/.exec(value);
  if (fromLink) {
    return fromLink[1];
  }
  return TOKEN_PATTERN.test(value) ? value : null;
}
