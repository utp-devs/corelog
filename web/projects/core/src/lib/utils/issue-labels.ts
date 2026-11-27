import { IssuePriority, IssueStatus, IssueType } from '../models/issue.model';

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

/** What is needed to label an issue: its server key when the payload carries one, else its id. */
export interface IssueKeyRef {
  readonly id: string;
  readonly key?: string | null;
}

/**
 * Visible key of an issue (for example CORE-12). The server key is used whenever the payload carries
 * it; payloads that only know the identifier fall back to a prefix of the id.
 */
export function issueKeyOf(issue: IssueKeyRef): string {
  return issue.key ? issue.key : `INC-${issue.id.slice(0, 8).toUpperCase()}`;
}

/** Compact key for dense lists: the server key when present, else a shorter prefix of the id. */
export function shortIssueKeyOf(issue: IssueKeyRef): string {
  return issue.key ? issue.key : `INC-${issue.id.slice(0, 6).toUpperCase()}`;
}

/** Visual category of a workflow category (the lozenge calls "in progress" `progress`). */
export function workflowCategoryVisual(
  category: 'todo' | 'in_progress' | 'done',
): IssueStatusCategory {
  return category === 'in_progress' ? 'progress' : category;
}

/** Status name and visual category of an issue: the workflow status when present, else the legacy value. */
export function issueStatusView(issue: {
  readonly status: IssueStatus;
  readonly workflowStatus?: {
    readonly name: string;
    readonly category: 'todo' | 'in_progress' | 'done';
  } | null;
}): IssueStatusMeta {
  if (issue.workflowStatus) {
    return {
      label: issue.workflowStatus.name,
      category: workflowCategoryVisual(issue.workflowStatus.category),
    };
  }
  return ISSUE_STATUS_META[issue.status];
}
