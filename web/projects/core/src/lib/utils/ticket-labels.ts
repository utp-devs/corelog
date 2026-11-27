import { IssuePriority, IssueStatus, IssueType } from '../models/ticket.model';

/** Visual category of a status badge. The UI maps each category to a design token. */
export type IssueStatusCategory = 'todo' | 'progress' | 'done';

export interface IssueStatusMeta {
  readonly label: string;
  readonly category: IssueStatusCategory;
}

export const ISSUE_STATUS_META: Readonly<Record<IssueStatus, IssueStatusMeta>> = {
  open: { label: 'Abierto', category: 'todo' },
  in_progress: { label: 'En progreso', category: 'progress' },
  resolved: { label: 'Resuelto', category: 'done' },
  closed: { label: 'Cerrado', category: 'done' },
};

export const ISSUE_PRIORITY_LABELS: Readonly<Record<IssuePriority, string>> = {
  low: 'Baja',
  medium: 'Media',
  high: 'Alta',
  critical: 'Crítica',
};

export const ISSUE_TYPE_LABELS: Readonly<Record<IssueType, string>> = {
  task: 'Tarea',
  bug: 'Error',
  story: 'Historia',
  epic: 'Épica',
  subtask: 'Subtarea',
};

/** Visible key of an issue. Issues have no own key, so it is derived from the identifier. */
export function issueKeyOf(id: string): string {
  return `INC-${id.slice(0, 8).toUpperCase()}`;
}
