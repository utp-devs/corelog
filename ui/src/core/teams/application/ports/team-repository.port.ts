import type { OrganizationPerson, Team, TeamMember, TeamMemberRole } from '@/core/teams/domain/team'

export interface CreateTeamInput {
  organizationId: string
  name: string
  description?: string
  avatarColor?: string | null
}

export interface UpdateTeamInput {
  name?: string
  description?: string
  avatarColor?: string | null
}

export interface TeamRepositoryPort {
  /** Teams of the current organization (the organization comes from the session context). */
  list(): Promise<Team[]>
  get(id: string): Promise<Team>
  create(input: CreateTeamInput): Promise<Team>
  update(id: string, input: UpdateTeamInput): Promise<Team>
  remove(id: string): Promise<void>
  listMembers(teamId: string): Promise<TeamMember[]>
  addMember(teamId: string, userId: string, role?: TeamMemberRole): Promise<TeamMember>
  updateMemberRole(teamId: string, userId: string, role: TeamMemberRole): Promise<TeamMember>
  removeMember(teamId: string, userId: string): Promise<void>
  /** People of an organization with their names. Guests may not read it (the server answers 403). */
  listOrganizationPeople(organizationId: string): Promise<OrganizationPerson[]>
}
