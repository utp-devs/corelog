export type TeamMemberRole = 'maintainer' | 'member'

export interface Team {
  id: string
  organizationId: string
  name: string
  description: string
  /** Hex color (#RRGGBB) or null when no color is set. */
  avatarColor: string | null
  /** Creator of the team, or null when unknown. */
  createdBy: string | null
  createdAt: string
  updatedAt: string
}

export interface TeamMember {
  teamId: string
  userId: string
  role: TeamMemberRole
  createdAt: string
}
