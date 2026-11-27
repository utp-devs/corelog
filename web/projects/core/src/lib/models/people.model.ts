import type { OrgRole } from './organization.model';

/** Member of the current organization, as listed by the people directory (name, job title, avatar flag). */
export interface PeopleMember {
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

/** Public profile of one member of the current organization. Includes the time zone. */
export interface PersonProfile {
  readonly userId: string;
  readonly name: string;
  readonly displayName: string;
  readonly email: string;
  readonly jobTitle: string;
  readonly department: string;
  readonly timezone: string;
  readonly hasAvatar: boolean;
  readonly orgRole: OrgRole;
  readonly joinedAt: string;
}

/** Client-side allowlist for avatar uploads. Must match the API rules. */
export const AVATAR_MAX_BYTES = 2 * 1024 * 1024;
export const AVATAR_MIME_TYPES: readonly string[] = ['image/png', 'image/jpeg', 'image/webp'];
