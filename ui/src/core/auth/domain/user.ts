/** Platform role. The first account of an installation is admin; every other account is user. */
export type Role = 'admin' | 'user'

export interface User {
  id: string
  name: string
  email: string
  role: Role
  teamId: string | null
  displayName: string
  jobTitle: string
  department: string
  avatarKey: string | null
  /** IANA time zone name, for example America/Bogota. Defaults to UTC. */
  timezone: string
  createdAt: string
  lastLoginAt: string | null
  emailVerifiedAt: string | null
}
