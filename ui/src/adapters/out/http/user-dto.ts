import type { Role, User } from '@/core/auth/domain/user'

export interface UserDto {
  id: string
  name: string
  email: string
  role: Role
  team_id?: string | null
  display_name?: string
  job_title?: string
  department?: string
  avatar_key?: string | null
  timezone?: string
  created_at: string
  last_login_at?: string | null
  email_verified_at?: string | null
}

export function toUser(dto: UserDto): User {
  return {
    id: dto.id,
    name: dto.name,
    email: dto.email,
    role: dto.role,
    teamId: dto.team_id ?? null,
    displayName: dto.display_name ?? '',
    jobTitle: dto.job_title ?? '',
    department: dto.department ?? '',
    avatarKey: dto.avatar_key ?? null,
    timezone: dto.timezone || 'UTC',
    createdAt: dto.created_at,
    lastLoginAt: dto.last_login_at ?? null,
    emailVerifiedAt: dto.email_verified_at ?? null,
  }
}

export function toUsers(dtos: UserDto[]): User[] {
  return dtos.map(toUser)
}
