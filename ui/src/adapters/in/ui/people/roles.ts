import type { LozengeTone } from '@/shared/ui/Lozenge'
import type { OrganizationMember, OrgRole } from '@/core/people/domain/member'

export const ORG_ROLE_LABELS: Record<OrgRole, string> = {
  owner: 'Propietario',
  admin: 'Administrador',
  member: 'Miembro',
  guest: 'Invitado',
}

export const ORG_ROLE_TONES: Record<OrgRole, LozengeTone> = {
  owner: 'info',
  admin: 'success',
  member: 'neutral',
  guest: 'warning',
}

/** Case-insensitive match against name and email. An empty query keeps every member. */
export function filterMembers(members: OrganizationMember[], query: string): OrganizationMember[] {
  const needle = query.trim().toLocaleLowerCase()
  if (!needle) return members
  return members.filter(
    (member) => member.name.toLocaleLowerCase().includes(needle) || member.email.toLocaleLowerCase().includes(needle),
  )
}
