export type TeamMemberRole = 'maintainer' | 'member'

/** Role of the caller inside a team; null when the caller is not a member. */
export type TeamMyRole = TeamMemberRole | null

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
  /** Number of members. Optional: absent when the API response does not carry it (merged, not reset, on update). */
  memberCount?: number
  /** Caller role in the team. Optional for the same reason as memberCount. */
  myRole?: TeamMyRole
}

export interface TeamMember {
  teamId: string
  userId: string
  role: TeamMemberRole
  createdAt: string
}

/** Person of the organization directory, used to show names next to team memberships. */
export interface OrganizationPerson {
  userId: string
  name: string
  email: string
}
