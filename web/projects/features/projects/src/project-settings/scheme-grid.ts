import { PermissionGrant, PermissionHolder } from '@corelog/core';

import { permissionCopy } from './permission-catalog';

/** Columns of the matrix: the roles first, then the holders that only apply on an issue. */
export interface HolderColumn {
  readonly key: PermissionHolder;
  readonly label: string;
  readonly hint: string;
  /** Special holders depend on the issue, not on the person's role in the project. */
  readonly special: boolean;
}

export const HOLDER_COLUMNS: readonly HolderColumn[] = [
  { key: 'lead', label: 'Líder', hint: 'Rol de líder del proyecto', special: false },
  { key: 'contributor', label: 'Colaborador', hint: 'Rol de colaborador', special: false },
  { key: 'viewer', label: 'Observador', hint: 'Rol de observador', special: false },
  {
    key: 'org_admin',
    label: 'Admin. de la organización',
    hint: 'Propietarios y administradores de la organización',
    special: false,
  },
  {
    key: 'reporter',
    label: 'Quien creó la incidencia',
    hint: 'Solo aplica sobre cada incidencia',
    special: true,
  },
  {
    key: 'assignee',
    label: 'Responsable',
    hint: 'Solo aplica sobre cada incidencia',
    special: true,
  },
  {
    key: 'watcher',
    label: 'Quien la sigue',
    hint: 'Solo aplica sobre cada incidencia',
    special: true,
  },
];

const HOLDER_ORDER: readonly PermissionHolder[] = HOLDER_COLUMNS.map((column) => column.key);

export function holderLabel(holder: PermissionHolder): string {
  return HOLDER_COLUMNS.find((column) => column.key === holder)?.label ?? holder;
}

/** Permission to holders, the working copy of a scheme. */
export type SchemeGrid = ReadonlyMap<string, ReadonlySet<PermissionHolder>>;

export function gridOf(grants: readonly PermissionGrant[]): SchemeGrid {
  return new Map(grants.map((grant) => [grant.permission, new Set(grant.holders)]));
}

/** Grants in the order of `order` (the permissions of the scheme), holders in column order. */
export function grantsOf(grid: SchemeGrid, order: readonly string[]): PermissionGrant[] {
  return order.map((permission) => ({
    permission,
    holders: HOLDER_ORDER.filter((holder) => grid.get(permission)?.has(holder)),
  }));
}

export function hasHolder(grid: SchemeGrid, permission: string, holder: PermissionHolder): boolean {
  return grid.get(permission)?.has(holder) ?? false;
}

export function toggleHolder(
  grid: SchemeGrid,
  permission: string,
  holder: PermissionHolder,
  on: boolean,
): SchemeGrid {
  const next = new Map(grid);
  const holders = new Set(grid.get(permission) ?? []);
  if (on) holders.add(holder);
  else holders.delete(holder);
  next.set(permission, holders);
  return next;
}

/** One permission whose holders changed. */
export interface GridChange {
  readonly permission: string;
  readonly label: string;
  readonly added: readonly PermissionHolder[];
  readonly removed: readonly PermissionHolder[];
}

/** Differences between two grids, in the order of `order`. */
export function diffGrids(
  before: SchemeGrid,
  after: SchemeGrid,
  order: readonly string[],
): GridChange[] {
  const changes: GridChange[] = [];
  for (const permission of order) {
    const was = before.get(permission) ?? new Set<PermissionHolder>();
    const now = after.get(permission) ?? new Set<PermissionHolder>();
    const added = HOLDER_ORDER.filter((holder) => now.has(holder) && !was.has(holder));
    const removed = HOLDER_ORDER.filter((holder) => was.has(holder) && !now.has(holder));
    if (added.length > 0 || removed.length > 0) {
      changes.push({ permission, label: permissionCopy(permission).label, added, removed });
    }
  }
  return changes;
}

/**
 * The default scheme of a project without rules of its own. It mirrors the server default so the page can
 * preview what restoring it changes; after restoring, the page shows the scheme the server answers.
 */
const DEFAULT_GRANTS: Readonly<Record<string, readonly PermissionHolder[]>> = {
  browse_project: ['lead', 'contributor', 'viewer'],
  create_issue: ['lead', 'contributor'],
  edit_issue: ['lead', 'contributor'],
  edit_own_issue: ['reporter'],
  delete_issue: ['lead'],
  assign_issue: ['lead', 'contributor'],
  transition_issue: ['lead', 'contributor', 'assignee'],
  comment_issue: ['lead', 'contributor'],
  edit_own_comment: ['lead', 'contributor'],
  delete_own_comment: ['lead', 'contributor'],
  manage_comments: ['lead'],
  add_attachment: ['lead', 'contributor'],
  delete_own_attachment: ['lead', 'contributor'],
  manage_attachments: ['lead'],
  link_issue: ['lead', 'contributor'],
  log_work: ['lead', 'contributor'],
  manage_worklogs: ['lead'],
  watch_issue: ['lead', 'contributor', 'viewer'],
  vote_issue: ['lead', 'contributor', 'viewer'],
  move_cards: ['lead', 'contributor'],
  manage_board: ['lead'],
  manage_sprints: ['lead'],
  manage_versions: ['lead'],
  manage_components: ['lead'],
  manage_workflow: ['lead'],
  manage_members: ['lead'],
  edit_project: ['lead'],
  delete_project: ['lead', 'org_admin'],
  view_reports: ['lead', 'contributor', 'viewer'],
  manage_automation: ['lead'],
  bulk_change: ['lead', 'contributor'],
  export_issues: ['lead', 'contributor', 'viewer'],
};

export function defaultGrid(order: readonly string[]): SchemeGrid {
  return new Map(
    order.map((permission) => [permission, new Set(DEFAULT_GRANTS[permission] ?? [])]),
  );
}

/** Permissions the lead must always keep, with the reason shown when the box is locked. */
export const LEAD_LOCKED: Readonly<Record<string, string>> = {
  manage_members: 'El líder siempre puede gestionar a las personas del proyecto.',
  edit_project: 'El líder siempre puede editar el proyecto.',
};

const VIEW_ROLES: readonly PermissionHolder[] = ['lead', 'contributor', 'viewer'];

/** Rule problems of a grid, in Spanish, before the server checks them. */
export function schemeProblems(grid: SchemeGrid): string[] {
  const problems: string[] = [];
  if (!hasHolder(grid, 'manage_members', 'lead')) {
    problems.push('El líder debe conservar el permiso para gestionar personas.');
  }
  if (!hasHolder(grid, 'edit_project', 'lead')) {
    problems.push('El líder debe conservar el permiso para editar el proyecto.');
  }
  if (
    !hasHolder(grid, 'delete_project', 'lead') &&
    !hasHolder(grid, 'delete_project', 'org_admin')
  ) {
    problems.push(
      'Alguien debe poder eliminar el proyecto: el líder o la administración de la organización.',
    );
  }
  if (!VIEW_ROLES.some((holder) => hasHolder(grid, 'browse_project', holder))) {
    problems.push('Algún rol debe poder ver el proyecto.');
  }
  return problems;
}
