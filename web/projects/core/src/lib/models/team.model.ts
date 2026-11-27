/** Role of a person inside a team. Maintainers manage the team; members take part in it. */
export type TeamRole = 'maintainer' | 'member';

/** Role of the caller inside a team; null when the caller is not a member. */
export type TeamMyRole = TeamRole | null;

export interface Team {
  id: string;
  organizationId: string;
  name: string;
  description: string;
  /** Hex color (#RRGGBB) or null when no color is set. */
  avatarColor: string | null;
  /** Creator of the team, or null when unknown. */
  createdBy: string | null;
  createdAt: string;
  updatedAt: string;
  /** Number of members. Absent when the API response does not carry it; kept, not reset, on update. */
  memberCount?: number;
  /** Caller role in the team. Optional for the same reason as memberCount. */
  myRole?: TeamMyRole;
}

export interface TeamMember {
  teamId: string;
  userId: string;
  role: TeamRole;
  createdAt: string;
}

/** Membership joined with the display data of the user (the membership endpoint only returns ids). */
export interface TeamMemberView extends TeamMember {
  name: string;
  email: string;
}

/** Person of the organization directory, used to show names next to team memberships. */
export interface OrganizationPerson {
  userId: string;
  name: string;
  email: string;
}

export interface CreateTeamInput {
  organizationId: string;
  name: string;
  description?: string;
  avatarColor?: string | null;
}

export interface UpdateTeamInput {
  name?: string;
  description?: string;
  avatarColor?: string | null;
}
