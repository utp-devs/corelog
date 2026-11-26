import type { User } from '@/core/auth/domain/user'
import type { OrganizationMember, PersonProfile } from '../../domain/member'

export interface PeopleRepositoryPort {
  listMembers(organizationId: string): Promise<OrganizationMember[]>
  /** Profile of one member. Rejects with 404 when the user is not a member, 403 for guests. */
  getPerson(organizationId: string, userId: string): Promise<PersonProfile>
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
