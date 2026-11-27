import { PermissionScheme, ProjectMemberRole } from '@corelog/core';

import { groupPermissions, permissionCopy } from '../permission-catalog';

export interface RoleAbility {
  readonly permission: string;
  readonly label: string;
  readonly description: string;
}

export interface RoleAbilityGroup {
  readonly key: string;
  readonly label: string;
  readonly abilities: readonly RoleAbility[];
}

export interface RoleSummary {
  readonly role: ProjectMemberRole;
  readonly total: number;
  readonly groups: readonly RoleAbilityGroup[];
}

/** What a role can do in the project, read from the permission scheme (never from a fixed table). */
export function summarizeRole(scheme: PermissionScheme, role: ProjectMemberRole): RoleSummary {
  const held = new Set(
    scheme.grants.filter((grant) => grant.holders.includes(role)).map((grant) => grant.permission),
  );
  const groups = groupPermissions(scheme.grants.map((grant) => grant.permission))
    .map((group) => ({
      key: group.key,
      label: group.label,
      abilities: group.permissions
        .filter((permission) => held.has(permission))
        .map((permission) => ({ permission, ...permissionCopy(permission) })),
    }))
    .filter((group) => group.abilities.length > 0);
  return {
    role,
    total: groups.reduce((sum, group) => sum + group.abilities.length, 0),
    groups,
  };
}
