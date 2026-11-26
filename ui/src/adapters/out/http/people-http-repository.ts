import type { User } from '@/core/auth/domain/user'
import type { PeopleRepositoryPort, ProfileRepositoryPort, ProfileUpdate } from '@/core/people/application/ports/people-repository.port'
import type { OrganizationMember, OrgRole, PersonProfile } from '@/core/people/domain/member'
import { apiClient } from './api-client'
import { toUser, type UserDto } from './user-dto'

interface MemberDto {
  user_id: string
  name: string
  display_name?: string
  email: string
  job_title?: string
  department?: string
  has_avatar?: boolean
  role: OrgRole
  joined_at: string
}

interface PersonDto {
  user_id: string
  name: string
  display_name?: string
  email: string
  job_title?: string
  department?: string
  timezone?: string
  has_avatar?: boolean
  org_role: OrgRole
  joined_at: string
}

/** Missing fields (older API) fall back to empty strings and false, never to undefined. */
function toMember(dto: MemberDto): OrganizationMember {
  return {
    userId: dto.user_id,
    name: dto.name,
    displayName: dto.display_name ?? '',
    email: dto.email,
    jobTitle: dto.job_title ?? '',
    department: dto.department ?? '',
    hasAvatar: dto.has_avatar ?? false,
    role: dto.role,
    joinedAt: dto.joined_at,
  }
}

function toPerson(dto: PersonDto): PersonProfile {
  return {
    userId: dto.user_id,
    name: dto.name,
    displayName: dto.display_name ?? '',
    email: dto.email,
    jobTitle: dto.job_title ?? '',
    department: dto.department ?? '',
    timezone: dto.timezone || 'UTC',
    hasAvatar: dto.has_avatar ?? false,
    orgRole: dto.org_role,
    joinedAt: dto.joined_at,
  }
}

export class PeopleHttpRepository implements PeopleRepositoryPort {
  async listMembers(organizationId: string): Promise<OrganizationMember[]> {
    const { data } = await apiClient.get<{ data: MemberDto[] }>(`/organizations/${encodeURIComponent(organizationId)}/members`)
    return data.data.map(toMember)
  }

  async getPerson(organizationId: string, userId: string): Promise<PersonProfile> {
    const { data } = await apiClient.get<{ data: PersonDto }>(
      `/organizations/${encodeURIComponent(organizationId)}/people/${encodeURIComponent(userId)}`,
    )
    return toPerson(data.data)
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
