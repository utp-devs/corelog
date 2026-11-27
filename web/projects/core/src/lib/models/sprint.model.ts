import type { WorkItemPriority, WorkItemStatus } from './board.model';

/** Lifecycle state of a sprint. Only one sprint per project can be active. */
export type SprintStatus = 'planned' | 'active' | 'closed';

/** Where the unfinished issues go when a sprint is closed. */
export type SprintUnfinishedPolicy = 'backlog' | 'sprint';

export interface Sprint {
  readonly id: string;
  readonly projectId: string;
  readonly name: string;
  readonly goal: string;
  readonly status: SprintStatus;
  readonly startDate?: string;
  readonly endDate?: string;
  readonly closedAt?: string;
}

/** Issue assigned to a sprint. */
export interface SprintIssue {
  readonly id: string;
  readonly title: string;
  readonly status: WorkItemStatus;
  readonly priority: WorkItemPriority;
  readonly storyPoints?: number;
  readonly closedAt?: string;
}

/** Remaining work of one day against the ideal line. Dates are AAAA-MM-DD. */
export interface BurndownPoint {
  readonly date: string;
  readonly ideal: number;
  readonly remaining: number;
}

/** Points completed by one closed sprint. */
export interface VelocityPoint {
  readonly sprintId: string;
  readonly name: string;
  readonly points: number;
}

/** Input for planning a sprint. Dates are optional and use AAAA-MM-DD. */
export interface CreateSprintInput {
  readonly name: string;
  readonly goal: string;
  readonly startDate?: string;
  readonly endDate?: string;
}
