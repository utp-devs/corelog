export type IssueStatus = 'open' | 'in_progress' | 'resolved' | 'closed';
export type IssuePriority = 'low' | 'medium' | 'high' | 'critical';
export type IssueType = 'task' | 'bug' | 'story' | 'epic' | 'subtask';
export type IssueLinkKind = 'blocks' | 'relates' | 'duplicates';

export const ISSUE_STATUSES: readonly IssueStatus[] = ['open', 'in_progress', 'resolved', 'closed'];
export const ISSUE_PRIORITIES: readonly IssuePriority[] = ['low', 'medium', 'high', 'critical'];
export const ISSUE_TYPES: readonly IssueType[] = ['task', 'bug', 'story', 'epic', 'subtask'];
export const ISSUE_LINK_KINDS: readonly IssueLinkKind[] = ['blocks', 'relates', 'duplicates'];

/** Full issue as returned by the API, with camelCase fields. */
export interface Ticket {
  readonly id: string;
  readonly title: string;
  readonly description: string;
  readonly status: IssueStatus;
  readonly priority: IssuePriority;
  readonly projectId: string;
  readonly issueType: IssueType;
  readonly parentId: string | null;
  readonly dueDate: string | null;
  readonly storyPoints: number | null;
  readonly rank: string;
  readonly closedAt: string | null;
  readonly requesterId: string;
  readonly requesterName: string;
  readonly assigneeId: string | null;
  readonly assigneeName: string | null;
  readonly createdAt: string;
  readonly updatedAt: string;
}

/** Transitions allowed by the issue workflow. */
const ALLOWED_TRANSITIONS: Readonly<Record<IssueStatus, readonly IssueStatus[]>> = {
  open: ['in_progress', 'closed'],
  in_progress: ['resolved', 'closed'],
  resolved: ['closed', 'in_progress'],
  closed: [],
};

export function canTransitionTo(current: IssueStatus, next: IssueStatus): boolean {
  return ALLOWED_TRANSITIONS[current].includes(next);
}

export interface IssueComment {
  readonly id: string;
  readonly issueId: string;
  readonly authorId: string;
  readonly body: string;
  readonly createdAt: string;
  readonly updatedAt: string;
  readonly deletedAt: string | null;
}

export interface IssueActivity {
  readonly id: string;
  readonly issueId: string;
  readonly actorId: string;
  /** Includes `assignee_changed` when the assignee was set or cleared. */
  readonly action: string;
  readonly fieldName: string | null;
  readonly oldValue: string | null;
  readonly newValue: string | null;
  readonly createdAt: string;
}

export interface IssueLink {
  readonly id: string;
  readonly sourceIssueId: string;
  readonly targetIssueId: string;
  readonly kind: IssueLinkKind;
  readonly createdBy: string;
  readonly createdAt: string;
}

export interface IssueAttachment {
  readonly id: string;
  readonly issueId: string;
  readonly uploaderId: string;
  readonly fileName: string;
  readonly contentType: string;
  readonly sizeBytes: number;
  readonly createdAt: string;
}

export interface IssueWatcherState {
  readonly isWatching: boolean;
  readonly count: number;
}

export interface SavedFilterCriteria {
  readonly status?: IssueStatus;
  readonly priority?: IssuePriority;
  readonly query?: string;
  readonly issueType?: IssueType;
  readonly dueFrom?: string;
  readonly dueTo?: string;
}

export interface SavedFilter {
  readonly id: string;
  readonly name: string;
  readonly criteria: SavedFilterCriteria;
  readonly createdAt: string;
  readonly updatedAt: string;
}

/** Filter and pagination of the issue list. Undefined fields are not sent. */
export interface ListTicketsFilter {
  readonly status?: IssueStatus;
  readonly priority?: IssuePriority;
  readonly query?: string;
  readonly issueType?: IssueType;
  readonly labelId?: string;
  readonly assigneeId?: string;
  readonly dueFrom?: string;
  readonly dueTo?: string;
  readonly page?: number;
  readonly pageSize?: number;
}

/** One page of issues and the total number of matches across all pages. */
export interface TicketPage {
  readonly items: readonly Ticket[];
  readonly total: number;
  readonly page: number;
  readonly pageSize: number;
}

export interface CreateTicketInput {
  /** Project the issue belongs to. Without it the server uses the default project. */
  readonly projectId?: string;
  readonly title: string;
  readonly description: string;
  readonly priority: IssuePriority;
  readonly issueType?: IssueType;
  readonly parentId?: string;
  readonly dueDate?: string;
  readonly storyPoints?: number;
}

export interface UpdateIssueMetadataInput {
  readonly issueType: IssueType;
  readonly parentId?: string;
  readonly dueDate?: string;
  readonly storyPoints?: number;
}

export interface UpdateIssueDetailsInput extends UpdateIssueMetadataInput {
  readonly title: string;
  readonly description: string;
  readonly priority: IssuePriority;
}
