import type {
  CreateTeamInput,
  TeamRepositoryPort,
  UpdateTeamInput,
} from '@/core/teams/application/ports/team-repository.port'
import type { Team, TeamMember, TeamMemberRole } from '@/core/teams/domain/team'
import { withOrg } from '@/shared/lib/with-org'
import { apiClient } from './api-client'

interface TeamDto {
  id: string
  organization_id: string
  name: string
  description: string
  avatar_color: string | null
  created_by: string | null
  created_at: string
  updated_at: string
}

interface TeamMemberDto {
  team_id: string
  user_id: string
  role: TeamMemberRole
  created_at: string
}

function toTeam(dto: TeamDto): Team {
  return {
    id: dto.id,
    organizationId: dto.organization_id,
    name: dto.name,
    description: dto.description ?? '',
    avatarColor: dto.avatar_color ?? null,
    createdBy: dto.created_by ?? null,
    createdAt: dto.created_at,
    updatedAt: dto.updated_at,
  }
}

function toTeamMember(dto: TeamMemberDto): TeamMember {
  return {
    teamId: dto.team_id,
    userId: dto.user_id,
    role: dto.role,
    createdAt: dto.created_at,
  }
}

export class TeamHttpRepository implements TeamRepositoryPort {
  async list(): Promise<Team[]> {
    const { data } = await apiClient.get<{ data: TeamDto[] }>('/teams', { params: withOrg() })
    return data.data.map(toTeam)
  }

  async get(id: string): Promise<Team> {
    const { data } = await apiClient.get<{ data: TeamDto }>(`/teams/${id}`)
    return toTeam(data.data)
  }

  async create(input: CreateTeamInput): Promise<Team> {
    const body: Record<string, unknown> = { organization_id: input.organizationId, name: input.name }
    if (input.description !== undefined) body.description = input.description
    if (input.avatarColor !== undefined) body.avatar_color = input.avatarColor
    const { data } = await apiClient.post<{ data: TeamDto }>('/teams', body)
    return toTeam(data.data)
  }

  async update(id: string, input: UpdateTeamInput): Promise<Team> {
    const body: Record<string, unknown> = {}
    if (input.name !== undefined) body.name = input.name
    if (input.description !== undefined) body.description = input.description
    if (input.avatarColor !== undefined) body.avatar_color = input.avatarColor
    const { data } = await apiClient.patch<{ data: TeamDto }>(`/teams/${id}`, body)
    return toTeam(data.data)
  }

  async remove(id: string): Promise<void> {
    await apiClient.delete(`/teams/${id}`)
  }

  async listMembers(teamId: string): Promise<TeamMember[]> {
    const { data } = await apiClient.get<{ data: TeamMemberDto[] }>(`/teams/${teamId}/members`)
    return data.data.map(toTeamMember)
  }

  async addMember(teamId: string, userId: string, role?: TeamMemberRole): Promise<TeamMember> {
    const body: Record<string, unknown> = { user_id: userId }
    if (role !== undefined) body.role = role
    const { data } = await apiClient.post<{ data: TeamMemberDto }>(`/teams/${teamId}/members`, body)
    return toTeamMember(data.data)
  }

  async updateMemberRole(teamId: string, userId: string, role: TeamMemberRole): Promise<TeamMember> {
    const { data } = await apiClient.patch<{ data: TeamMemberDto }>(`/teams/${teamId}/members/${userId}`, { role })
    return toTeamMember(data.data)
  }

  async removeMember(teamId: string, userId: string): Promise<void> {
    await apiClient.delete(`/teams/${teamId}/members/${userId}`)
  }
}
