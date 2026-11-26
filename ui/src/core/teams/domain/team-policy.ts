import type { OrgRole, OrgWhoCan } from '@/core/organizations/domain/organization'
import type { Team } from './team'

/**
 * Whether the caller may create teams in the organization. Owners and admins always may; members only when
 * the organization policy allows members; guests never. Mirrors the server rule (team.creation_not_allowed).
 */
export function canCreateTeam(role: OrgRole | null | undefined, policy: OrgWhoCan | undefined): boolean {
  if (role === 'owner' || role === 'admin') return true
  if (role === 'member') return policy === 'members'
  return false
}

/**
 * Whether the caller may edit, delete or manage the members of a team: organization owners and admins, or a
 * maintainer of that team. Server stays the authority (team.forbidden).
 */
export function canManageTeam(orgRole: OrgRole | null | undefined, viewerTeamRole: string | null | undefined): boolean {
  if (orgRole === 'owner' || orgRole === 'admin') return true
  return viewerTeamRole === 'maintainer'
}

/** Client-side search over the name and description of teams (case and accent insensitive). */
export function filterTeams(teams: Team[], query: string): Team[] {
  const needle = normalizeText(query)
  if (needle === '') return teams
  return teams.filter((team) => normalizeText(`${team.name} ${team.description}`).includes(needle))
}

function normalizeText(value: string): string {
  return value
    .normalize('NFD')
    .replace(/[̀-ͯ]/g, '')
    .toLowerCase()
    .trim()
}
