import type { StatusCategory } from './board-v2.model';
import type { AgileSprint } from './sprint-v2.model';

export interface BacklogLabel {
  readonly id: string;
  readonly name: string;
  readonly color: string;
}

export interface BacklogIssue {
  readonly id: string;
  readonly key: string;
  readonly summary: string;
  readonly issueType: string;
  readonly issueTypeName: string;
  readonly issueTypeIcon: string;
  readonly issueTypeColor: string;
  readonly status: string;
  readonly statusName: string;
  readonly statusCategory: StatusCategory;
  readonly priority: string;
  readonly priorityName: string;
  readonly priorityIcon: string;
  readonly priorityColor: string;
  readonly assigneeId: string | null;
  readonly assigneeName: string;
  readonly storyPoints: number | null;
  readonly originalEstimateSeconds: number | null;
  readonly rank: string;
  readonly epicId: string | null;
  readonly epicKey: string;
  readonly epicSummary: string;
  readonly flagged: boolean;
  readonly labels: readonly BacklogLabel[];
}

export interface BacklogCategoryTotals {
  readonly issues: number;
  readonly points: number;
  readonly originalEstimateSeconds: number;
}

export interface BacklogTotals extends BacklogCategoryTotals {
  readonly byCategory: Readonly<Partial<Record<StatusCategory, BacklogCategoryTotals>>>;
}

export type BacklogSectionKind = 'active' | 'planned' | 'backlog';

export interface BacklogSection {
  readonly kind: BacklogSectionKind;
  /** Null in the backlog section. */
  readonly sprint: AgileSprint | null;
  readonly issues: readonly BacklogIssue[];
  readonly totals: BacklogTotals;
}

export interface BacklogEpic {
  readonly id: string;
  readonly key: string;
  readonly summary: string;
  readonly status: StatusCategory;
  readonly total: number;
  readonly done: number;
  readonly points: number;
  readonly donePoints: number;
  /** Fraction of finished children, from 0 to 1. */
  readonly progress: number;
}

export interface BacklogView {
  readonly projectId: string;
  readonly sections: readonly BacklogSection[];
  readonly epics: readonly BacklogEpic[];
  readonly canManageSprints: boolean;
  readonly canMoveCards: boolean;
  readonly truncated: boolean;
}

export interface BacklogQuery {
  readonly epic?: string;
  readonly q?: string;
}

/** Moves issues, in the given order, to a section next to an anchor issue. */
export interface RankBacklogInput {
  readonly issueIds: readonly string[];
  /** Sprint id of the target section; null for the backlog. */
  readonly sprintId: string | null;
  readonly beforeIssueId?: string;
  readonly afterIssueId?: string;
}
