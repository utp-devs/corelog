import type { StatusCategory } from './board-v2.model';
import type { SprintStatus } from './sprint.model';

/** Sprint with the scope numbers the planning screens show. */
export interface AgileSprint {
  readonly id: string;
  readonly projectId: string;
  readonly boardId: string | null;
  readonly name: string;
  readonly goal: string;
  readonly status: SprintStatus;
  readonly startDate: string | null;
  readonly endDate: string | null;
  readonly startedAt: string | null;
  readonly completedAt: string | null;
  readonly committedPoints: number | null;
  readonly completedPoints: number | null;
}

export interface CreateAgileSprintInput {
  readonly name: string;
  readonly goal?: string;
  readonly startDate?: string | null;
  readonly endDate?: string | null;
}

export interface UpdateAgileSprintInput {
  readonly name?: string;
  readonly goal?: string;
  readonly startDate?: string | null;
  readonly endDate?: string | null;
}

export interface StartSprintInput {
  readonly name?: string;
  readonly goal?: string;
  readonly startDate?: string;
  readonly endDate?: string;
  /** Allows starting a sprint without issues. */
  readonly force?: boolean;
}

/** Where the unfinished issues go: the backlog, a planned sprint (`sprint:<id>`) or a new sprint. */
export type SprintDestination = 'backlog' | 'new' | `sprint:${string}`;

export interface CompleteSprintInput {
  readonly destination: SprintDestination;
  readonly newSprintName?: string;
}

export interface SprintCompletion {
  readonly sprint: AgileSprint;
  readonly nextSprint: AgileSprint | null;
  readonly completedIssues: number;
  readonly movedIssues: number;
  readonly completedPoints: number;
  readonly committedPoints: number;
}

export interface SprintReportIssue {
  readonly issueId: string;
  readonly key: string;
  readonly summary: string;
  readonly points: number | null;
  readonly statusCategory: StatusCategory;
  readonly addedAfterStart: boolean;
  readonly removedAt?: string;
  readonly removedKind?: 'removed' | 'moved_out';
}

export interface SprintReportBurndownPoint {
  readonly date: string;
  readonly ideal: number | null;
  readonly remaining: number;
  readonly scope: number;
  readonly completed: number;
}

export interface SprintVelocityEntry {
  readonly sprintId: string;
  readonly name: string;
  readonly points: number;
}

export interface SprintReport {
  readonly sprint: AgileSprint;
  readonly committedPoints: number;
  readonly completedPoints: number;
  readonly incompletePoints: number;
  readonly addedPoints: number;
  readonly removedPoints: number;
  readonly completed: readonly SprintReportIssue[];
  readonly incomplete: readonly SprintReportIssue[];
  readonly removed: readonly SprintReportIssue[];
  readonly addedAfterStart: readonly SprintReportIssue[];
  readonly burndown: readonly SprintReportBurndownPoint[];
  readonly velocity: readonly SprintVelocityEntry[];
}
