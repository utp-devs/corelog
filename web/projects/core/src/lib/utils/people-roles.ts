import type { OrgRole } from '../models/organization.model';
import type { PeopleMember } from '../models/people.model';

/** Visual tone of a role badge. Mapped to the badge component by the feature that renders it. */
export type OrgRoleTone = 'info' | 'success' | 'neutral' | 'warning';

export const ORG_ROLE_LABELS: Readonly<Record<OrgRole, string>> = {
  owner: 'Propietario',
  admin: 'Administrador',
  member: 'Miembro',
  guest: 'Invitado',
};

export const ORG_ROLE_TONES: Readonly<Record<OrgRole, OrgRoleTone>> = {
  owner: 'info',
  admin: 'success',
  member: 'neutral',
  guest: 'warning',
};

/** Case-insensitive match against name and email. An empty query keeps every member. */
export function filterMembers(members: readonly PeopleMember[], query: string): PeopleMember[] {
  const needle = query.trim().toLocaleLowerCase();
  if (!needle) return [...members];
  return members.filter(
    (member) =>
      member.name.toLocaleLowerCase().includes(needle) ||
      member.email.toLocaleLowerCase().includes(needle),
  );
}
