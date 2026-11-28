/** Category of a workflow status: the three groups a board and a backlog total by. */
export type StatusCategory = 'todo' | 'in_progress' | 'done';

export type BoardSwimlaneMode = 'none' | 'assignee' | 'epic' | 'query' | 'story';

/** What a column header adds up: story points, number of issues or remaining time. */
export type BoardEstimation = 'story_points' | 'issue_count' | 'time';

/** Fields a card can show. */
export type BoardCardField =
  | 'key'
  | 'type'
  | 'priority'
  | 'assignee'
  | 'labels'
  | 'points'
  | 'epic'
  | 'subtasks'
  | 'flag'
  | 'estimate';

/** State of a column against its work-in-progress limits. Advisory: it never blocks a move. */
export type WipState = 'ok' | 'below_min' | 'at_max' | 'over_max';

/** Colors a column can take. An empty color means the default. */
export const BOARD_COLUMN_COLORS = [
  'gray',
  'red',
  'orange',
  'amber',
  'green',
  'teal',
  'blue',
  'indigo',
  'purple',
  'pink',
] as const;

export interface BoardWorkflowStatus {
  readonly id: string;
  readonly name: string;
  readonly category: StatusCategory;
}

/** Column as configured on the board (settings), with the workflow statuses it holds. */
export interface BoardColumnConfig {
  readonly id: string;
  readonly name: string;
  readonly color: string;
  readonly position: number;
  readonly minIssues: number | null;
  readonly maxIssues: number | null;
  readonly statusIds: readonly string[];
}

/** Saved query of a board: a quick filter or a query swimlane. */
export interface BoardQuery {
  readonly id: string;
  readonly name: string;
  readonly query: string;
}

export interface BoardSummary {
  readonly id: string;
  readonly projectId: string;
  readonly name: string;
  readonly type: 'scrum' | 'kanban';
  readonly isDefault: boolean;
  readonly columnCount: number;
  readonly swimlaneMode: BoardSwimlaneMode;
}

/** Board with its full configuration. */
export interface BoardDetail {
  readonly id: string;
  readonly projectId: string;
  readonly name: string;
  readonly type: 'scrum' | 'kanban';
  readonly isDefault: boolean;
  readonly swimlaneMode: BoardSwimlaneMode;
  readonly estimation: BoardEstimation;
  readonly cardFields: readonly BoardCardField[];
  readonly showUnmappedLane: boolean;
  readonly doneRetentionDays: number | null;
  readonly columns: readonly BoardColumnConfig[];
  readonly quickFilters: readonly BoardQuery[];
  readonly swimlanes: readonly BoardQuery[];
  readonly workflowStatuses: readonly BoardWorkflowStatus[];
  readonly unmappedStatusIds: readonly string[];
  readonly canManageBoard: boolean;
  readonly canMoveCards: boolean;
}

/** Name with icon and color of a type or a priority, as the card shows it. */
export interface BoardNamed {
  readonly name: string;
  readonly key: string;
  readonly icon: string;
  readonly color: string;
}

export interface BoardRef {
  readonly id: string;
  readonly key: string;
  readonly summary: string;
}

export interface BoardViewCard {
  readonly id: string;
  readonly key: string;
  readonly summary: string;
  readonly type: BoardNamed;
  readonly priority: BoardNamed;
  readonly status: BoardWorkflowStatus;
  readonly assignee: { readonly id: string; readonly name: string } | null;
  readonly labels: readonly string[];
  readonly storyPoints: number | null;
  readonly remainingEstimateSeconds: number | null;
  readonly flagged: boolean;
  readonly rank: string;
  readonly sprintId: string | null;
  readonly epic: BoardRef | null;
  readonly parent: BoardRef | null;
  readonly subtasks: { readonly total: number; readonly done: number };
  /** Lane the card sits in; empty without swimlanes. */
  readonly swimlaneId: string;
}

export interface BoardViewColumn extends BoardColumnConfig {
  /** Cards shown after the quick filters. */
  readonly count: number;
  /** Cards of the column without quick filters; the WIP state is evaluated on it. */
  readonly total: number;
  readonly estimate: number;
  readonly wipState: WipState;
  readonly cards: readonly BoardViewCard[];
}

/** Implicit column with the cards whose status no column holds. */
export interface BoardViewUnmapped {
  readonly id: string;
  readonly name: string;
  readonly statusIds: readonly string[];
  readonly count: number;
  readonly total: number;
  readonly estimate: number;
  readonly cards: readonly BoardViewCard[];
}

export interface BoardViewSwimlane {
  readonly id: string;
  readonly name: string;
  readonly kind: BoardSwimlaneMode;
  readonly count: number;
}

export interface BoardView {
  readonly board: BoardDetail;
  readonly sprint: { readonly id: string; readonly name: string } | null;
  readonly columns: readonly BoardViewColumn[];
  readonly unmappedColumn: BoardViewUnmapped | null;
  readonly unmappedCount: number;
  readonly swimlanes: readonly BoardViewSwimlane[];
}

/** Which issues the view shows: `active`, `all`, `none` (no sprint) or a sprint id. */
export type BoardSprintFilter = string;

export interface BoardViewRequest {
  readonly quickFilterIds: readonly string[];
  readonly sprint?: BoardSprintFilter;
}

export interface BoardMoveInput {
  readonly columnId: string;
  readonly beforeIssueId?: string;
  readonly afterIssueId?: string;
  readonly swimlane?: string;
  readonly transitionId?: string;
}

export interface BoardWipResult {
  readonly columnId: string;
  readonly count: number;
  readonly minIssues: number | null;
  readonly maxIssues: number | null;
  readonly state: WipState;
  readonly exceeded: boolean;
}

export interface BoardMoveResult {
  readonly issueId: string;
  readonly columnId: string;
  readonly status: BoardWorkflowStatus;
  readonly transitionId: string;
  readonly transitioned: boolean;
  readonly rank: string;
  readonly wip: BoardWipResult | null;
}

export interface BoardColumnInput {
  readonly id?: string;
  readonly name: string;
  readonly color: string;
  readonly minIssues: number | null;
  readonly maxIssues: number | null;
  readonly statusIds: readonly string[];
}

export interface BoardQueryInput {
  readonly name: string;
  readonly query: string;
}

export interface CreateBoardInput {
  readonly name: string;
  readonly type?: 'scrum' | 'kanban';
}

export interface UpdateBoardInput {
  readonly name?: string;
  readonly swimlaneMode?: BoardSwimlaneMode;
  readonly estimation?: BoardEstimation;
  readonly cardFields?: readonly BoardCardField[];
  readonly showUnmappedLane?: boolean;
  readonly doneRetentionDays?: number | null;
}

/** Candidate transition when several lead into the target column. */
export interface BoardTransitionChoice {
  readonly id: string;
  readonly name: string;
}
