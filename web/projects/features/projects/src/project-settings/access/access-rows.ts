import { ProjectMember, ProjectMemberRole, ProjectTeamRole } from '@corelog/core';

import { SettingsPerson } from '../project-settings-context';
import { ROLE_ORDER } from '../settings-copy';

/** A person of the access table. */
export interface AccessRow {
  readonly userId: string;
  readonly name: string;
  readonly email: string;
  readonly hasAvatar: boolean;
  /** Role given to the person directly; null when they only reach the project through a team. */
  readonly directRole: ProjectMemberRole | null;
  /** Best role across the direct one and the teams. */
  readonly effectiveRole: ProjectMemberRole;
  readonly teamName: string | null;
}

/** A team of the access table. */
export interface TeamAccessRow {
  readonly teamId: string;
  readonly name: string;
  readonly role: ProjectMemberRole;
}

/** Table rows of the members: leads first, then by name. */
export function accessRowsOf(
  members: readonly ProjectMember[],
  personOf: (userId: string) => SettingsPerson,
): AccessRow[] {
  return members
    .map((member): AccessRow => {
      const person = personOf(member.userId);
      const viaTeamOnly = member.viaTeam === true;
      return {
        userId: member.userId,
        name: person.name,
        email: person.email,
        hasAvatar: person.hasAvatar,
        directRole: viaTeamOnly ? null : member.role,
        effectiveRole: member.effectiveRole ?? member.role,
        teamName: member.teamName ?? null,
      };
    })
    .sort(
      (a, b) =>
        ROLE_ORDER.indexOf(a.effectiveRole) - ROLE_ORDER.indexOf(b.effectiveRole) ||
        a.name.localeCompare(b.name, 'es'),
    );
}

export function teamRowsOf(teamRoles: readonly ProjectTeamRole[]): TeamAccessRow[] {
  return teamRoles
    .map((item) => ({ teamId: item.teamId, name: item.teamName, role: item.role }))
    .sort((a, b) => a.name.localeCompare(b.name, 'es'));
}
