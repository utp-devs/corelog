import { IssueTypeRef, PriorityRef, ResolutionRef } from './catalog.model';
import { WorkflowStatusRef } from './workflow.model';

export type IssueStatus = 'open' | 'in_progress' | 'resolved' | 'closed';
/** Legacy priority key, still used by the list filters and the board, backlog and sprint payloads. */
export type IssuePriority = 'low' | 'medium' | 'high' | 'critical';
/** Legacy issue type key, still used by the list filters and the board, backlog and sprint payloads. */
export type IssueType = 'task' | 'bug' | 'story' | 'epic' | 'subtask';
export type IssueLinkKind = 'blocks' | 'relates' | 'duplicates';

export const ISSUE_STATUSES: readonly IssueStatus[] = ['open', 'in_progress', 'resolved', 'closed'];
export const ISSUE_PRIORITIES: readonly IssuePriority[] = ['low', 'medium', 'high', 'critical'];
export const ISSUE_TYPES: readonly IssueType[] = ['task', 'bug', 'story', 'epic', 'subtask'];
export const ISSUE_LINK_KINDS: readonly IssueLinkKind[] = ['blocks', 'relates', 'duplicates'];

/** Full issue as returned by the API, with camelCase fields. */
export interface Issue {
  readonly id: string;
  readonly key: string;
  readonly projectKey: string;
  readonly summary: string;
  readonly description: string;
  /** Legacy mirror of the workflow status category (todo is open, in progress, done is closed). */
  readonly status: IssueStatus;
  /** Workflow status of the issue. Absent only in payloads of older endpoints. */
  readonly workflowStatus?: WorkflowStatusRef | null;
  readonly priority: PriorityRef;
  readonly projectId: string;
  readonly issueType: IssueTypeRef;
  readonly parentId: string | null;
  readonly dueDate: string | null;
  readonly storyPoints: number | null;
  readonly rank: string;
  readonly closedAt: string | null;
  /** Set when the issue is done; null while it is open. */
  readonly resolution: ResolutionRef | null;
  readonly resolvedAt: string | null;
  readonly reporterId: string;
  readonly reporterName: string;
  readonly assigneeId: string | null;
  readonly assigneeName: string | null;
  readonly createdAt: string;
  readonly updatedAt: string;
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
/** Value of the assignee filter that selects the issues nobody is assigned to. */
export const UNASSIGNED_FILTER = 'none';

export interface ListIssuesFilter {
  readonly status?: IssueStatus;
  readonly priority?: IssuePriority;
  readonly query?: string;
  readonly issueType?: IssueType;
  readonly labelId?: string;
  /** A user id, or `UNASSIGNED_FILTER` for the issues with no assignee. */
  readonly assigneeId?: string;
  readonly dueFrom?: string;
  readonly dueTo?: string;
  readonly page?: number;
  readonly pageSize?: number;
}

/** One page of issues and the total number of matches across all pages. */
export interface IssuePage {
  readonly items: readonly Issue[];
  readonly total: number;
  readonly page: number;
  readonly pageSize: number;
}

export interface CreateIssueInput {
  /** Project the issue belongs to. Without it the server uses the default project. */
  readonly projectId?: string;
  readonly summary: string;
  readonly description: string;
  /** Catalog priority id. Without it the server uses the default priority. */
  readonly priorityId?: string;
  /** Catalog issue type id. Without it the server uses the default type. */
  readonly issueTypeId?: string;
  readonly parentId?: string;
  readonly dueDate?: string;
  readonly storyPoints?: number;
}

export interface UpdateIssueMetadataInput {
  readonly issueTypeId: string;
  readonly parentId?: string;
  readonly dueDate?: string;
  readonly storyPoints?: number;
}

export interface UpdateIssueDetailsInput extends UpdateIssueMetadataInput {
  readonly summary: string;
  readonly description: string;
  readonly priorityId: string;
}
