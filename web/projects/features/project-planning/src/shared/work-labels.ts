import {
  AppRole,
  EffectiveProjectRole,
  ISSUE_TYPE_LABELS,
  IssueType,
  OrgRole,
  ProjectMember,
  WorkItemStatus,
  effectiveProjectRole,
  issueKeyOf,
} from '@corelog/core';
import { LozengeCategory } from '@corelog/ui/lozenge';

/** Names of the workflow states as the planning screens show them, chosen by status code. */
export const STATUS_NAMES: Readonly<Record<WorkItemStatus, string>> = {
  open: 'Abierto',
  in_progress: 'En curso',
  resolved: 'Resuelto',
  closed: 'Cerrado',
};

export const STATUS_CATEGORIES: Readonly<Record<WorkItemStatus, LozengeCategory>> = {
  open: 'todo',
  in_progress: 'progress',
  resolved: 'done',
  closed: 'done',
};

/** Spanish name of a status code. Unknown codes fall back to the given text. */
export function statusName(status: WorkItemStatus, fallback = ''): string {
  return STATUS_NAMES[status] ?? fallback;
}

/** Spanish name of an issue type. Unknown types fall back to the given text. */
export function typeName(type: string): string {
  return ISSUE_TYPE_LABELS[type as IssueType] ?? type;
}

/** Readable short key of an issue: the prefix and the first six characters of its identifier. */
export function shortKeyOf(issueId: string): string {
  const full = issueKeyOf(issueId);
  const dash = full.indexOf('-');
  return dash < 0 ? full : full.slice(0, dash + 7);
}

/**
 * Whether the caller can only look at the project: a project viewer, or an organization guest who
 * is not a lead or contributor of the project. Everybody else keeps the editing controls.
 */
export function isReadOnlyRole(
  globalRole: AppRole | string | undefined,
  orgRole: OrgRole | null | undefined,
  userId: string | undefined,
  members: readonly ProjectMember[],
): boolean {
  const role: EffectiveProjectRole = effectiveProjectRole(globalRole, userId, [...members]);
  if (role === 'viewer') return true;
  return orgRole === 'guest' && role !== 'lead' && role !== 'contributor' && role !== 'admin';
}
