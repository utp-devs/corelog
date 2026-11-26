import type { User } from '@/core/auth/domain/user'
import type { OrganizationMember } from '../../domain/member'

export interface PeopleRepositoryPort {
  listMembers(organizationId: string): Promise<OrganizationMember[]>
  /** Members of the organization whose name or email matches the query (server caps at 20). */
  searchUsers(organizationId: string, query: string): Promise<User[]>
}

export interface ProfileUpdate {
  name?: string
  displayName?: string
  jobTitle?: string
  department?: string
  timezone?: string
}

export interface ProfileRepositoryPort {
  getMe(): Promise<User>
  updateMe(payload: ProfileUpdate): Promise<User>
}
