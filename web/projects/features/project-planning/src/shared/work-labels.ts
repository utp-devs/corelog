import {
  AppRole,
  EffectiveProjectRole,
  ISSUE_TYPE_LABELS,
  IssueType,
  OrgRole,
  ProjectMember,
  Sprint,
  SprintStatus,
  WorkItemStatus,
  effectiveProjectRole,
} from '@corelog/core';
import { LozengeCategory, LozengeTone } from '@corelog/ui/lozenge';

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

/** Whether work on an item is over: resolved or closed. */
export function isResolvedStatus(status: WorkItemStatus): boolean {
  return status === 'resolved' || status === 'closed';
}

/** Spanish name of each sprint state. */
export const SPRINT_STATUS_LABELS: Readonly<Record<SprintStatus, string>> = {
  planned: 'Planificado',
  active: 'Activo',
  closed: 'Cerrado',
};

export const SPRINT_STATUS_TONES: Readonly<Record<SprintStatus, LozengeTone>> = {
  planned: 'info',
  active: 'success',
  closed: 'neutral',
};

/** ISO day (2026-11-19) as dd/MM/yyyy. Other text is returned unchanged. */
export function formatDay(value: string): string {
  const match = /^(\d{4})-(\d{2})-(\d{2})/.exec(value);
  return match ? `${match[3]}/${match[2]}/${match[1]}` : value;
}

/** Dates of a sprint as dd/MM/yyyy - dd/MM/yyyy, or a note when it has none. */
export function sprintDatesOf(sprint: Sprint): string {
  return sprint.startDate && sprint.endDate
    ? `${formatDay(sprint.startDate)} - ${formatDay(sprint.endDate)}`
    : 'Sin fechas';
}

/** Spanish name of a status code. Unknown codes fall back to the given text. */
export function statusName(status: WorkItemStatus, fallback = ''): string {
  return STATUS_NAMES[status] ?? fallback;
}

/** Spanish name of an issue type. Unknown types fall back to the given text. */
export function typeName(type: string): string {
  return ISSUE_TYPE_LABELS[type as IssueType] ?? type;
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
