import type { User } from '@/core/auth/domain/user'
import type { PeopleRepositoryPort, ProfileRepositoryPort, ProfileUpdate } from '@/core/people/application/ports/people-repository.port'
import type { OrganizationMember, OrgRole } from '@/core/people/domain/member'
import { apiClient } from './api-client'
import { toUser, type UserDto } from './user-dto'

interface MemberDto {
  user_id: string
  name: string
  email: string
  role: OrgRole
  joined_at: string
}

function toMember(dto: MemberDto): OrganizationMember {
  return { userId: dto.user_id, name: dto.name, email: dto.email, role: dto.role, joinedAt: dto.joined_at }
}

export class PeopleHttpRepository implements PeopleRepositoryPort {
  async listMembers(organizationId: string): Promise<OrganizationMember[]> {
    const { data } = await apiClient.get<{ data: MemberDto[] }>(`/organizations/${organizationId}/members`)
    return data.data.map(toMember)
  }

  async searchUsers(organizationId: string, query: string): Promise<User[]> {
    const { data } = await apiClient.get<{ data: UserDto[] }>('/users/search', {
      params: { q: query, organization_id: organizationId },
    })
    return data.data.map(toUser)
  }
}

function toProfileBody(payload: ProfileUpdate): Record<string, string> {
  const body: Record<string, string> = {}
  if (payload.name !== undefined) body.name = payload.name
  if (payload.displayName !== undefined) body.display_name = payload.displayName
  if (payload.jobTitle !== undefined) body.job_title = payload.jobTitle
  if (payload.department !== undefined) body.department = payload.department
  if (payload.timezone !== undefined) body.timezone = payload.timezone
  return body
}

export class ProfileHttpRepository implements ProfileRepositoryPort {
  async getMe(): Promise<User> {
    const { data } = await apiClient.get<{ data: UserDto }>('/users/me')
    return toUser(data.data)
  }

  /** Partial update: only the fields present in the payload are sent. */
  async updateMe(payload: ProfileUpdate): Promise<User> {
    const { data } = await apiClient.patch<{ data: UserDto }>('/users/me', toProfileBody(payload))
    return toUser(data.data)
  }
}
