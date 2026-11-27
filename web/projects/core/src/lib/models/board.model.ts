/** Status of a work item as exposed by the board, backlog, sprint and dashboard endpoints. */
export type WorkItemStatus = 'open' | 'in_progress' | 'resolved' | 'closed';

/** Priority of a work item, from the least to the most urgent in the API enum order. */
export type WorkItemPriority = 'low' | 'medium' | 'high' | 'critical';

/** Board flavour returned by the API. */
export type BoardType = 'kanban' | 'scrum';

/** Card shown in a board column. */
export interface BoardCard {
  readonly id: string;
  readonly type: string;
  readonly summary: string;
  readonly status: WorkItemStatus;
  readonly priority: WorkItemPriority;
  readonly assigneeId?: string;
  readonly assignee?: string;
  readonly storyPoints?: number;
  readonly rank: string;
}

/** Column of a board. `wipLimit` is undefined when the column has no work-in-progress limit. */
export interface BoardColumn {
  readonly id: string;
  readonly name: string;
  readonly status: WorkItemStatus;
  readonly position: number;
  readonly wipLimit?: number;
  readonly count: number;
  readonly cards: readonly BoardCard[];
}

/** Default board of a project with its columns. */
export interface ProjectBoard {
  readonly id: string;
  readonly projectId: string;
  readonly name: string;
  readonly type: BoardType;
  readonly columns: readonly BoardColumn[];
  readonly canManageBoard: boolean;
}

/** Quick filters applied on the loaded cards. They never trigger a request. */
export interface BoardFilter {
  readonly search: string;
  readonly priority: WorkItemPriority | null;
  readonly onlyMine: boolean;
}
