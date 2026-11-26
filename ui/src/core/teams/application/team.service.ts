import type { OrganizationPerson, Team, TeamMember, TeamMemberRole } from '../domain/team'
import type { CreateTeamInput, TeamRepositoryPort, UpdateTeamInput } from './ports/team-repository.port'

export class TeamService {
  private readonly repository: TeamRepositoryPort

  constructor(repository: TeamRepositoryPort) {
    this.repository = repository
  }

  list(): Promise<Team[]> {
    return this.repository.list()
  }

  get(id: string): Promise<Team> {
    return this.repository.get(id)
  }

  create(input: CreateTeamInput): Promise<Team> {
    return this.repository.create(input)
  }

  update(id: string, input: UpdateTeamInput): Promise<Team> {
    return this.repository.update(id, input)
  }

  remove(id: string): Promise<void> {
    return this.repository.remove(id)
  }

  listMembers(teamId: string): Promise<TeamMember[]> {
    return this.repository.listMembers(teamId)
  }

  addMember(teamId: string, userId: string, role?: TeamMemberRole): Promise<TeamMember> {
    return this.repository.addMember(teamId, userId, role)
  }

  updateMemberRole(teamId: string, userId: string, role: TeamMemberRole): Promise<TeamMember> {
    return this.repository.updateMemberRole(teamId, userId, role)
  }

  removeMember(teamId: string, userId: string): Promise<void> {
    return this.repository.removeMember(teamId, userId)
  }

  listOrganizationPeople(organizationId: string): Promise<OrganizationPerson[]> {
    return this.repository.listOrganizationPeople(organizationId)
  }
}
