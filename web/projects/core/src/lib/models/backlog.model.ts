import type { WorkItemPriority, WorkItemStatus } from './board.model';

/** Item of the project backlog, ordered by rank (the first item is worked first). */
export interface BacklogItem {
  readonly id: string;
  readonly projectId: string;
  readonly summary: string;
  readonly type: string;
  readonly status: WorkItemStatus;
  readonly priority: WorkItemPriority;
  readonly rank: string;
  readonly assigneeId?: string;
  readonly storyPoints?: number;
}
