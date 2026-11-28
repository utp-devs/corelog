import { Issue } from './issue.model';
import { IssueTransition } from './workflow.model';

/** Effective permissions of the viewer on one issue, as named by the API. */
export type IssuePermission =
  | 'edit_issue'
  | 'assign_issue'
  | 'transition_issue'
  | 'comment_issue'
  | 'delete_issue'
  | 'add_attachment'
  | 'link_issue'
  | 'watch_issue'
  | 'vote_issue'
  | 'log_work'
  | 'manage_worklogs'
  | 'manage_comments'
  | 'create_issue';

export type CustomFieldType =
  | 'text'
  | 'paragraph'
  | 'number'
  | 'date'
  | 'datetime'
  | 'select'
  | 'multiselect'
  | 'checkbox'
  | 'user'
  | 'url'
  | 'labels';

export const CUSTOM_FIELD_TYPES: readonly CustomFieldType[] = [
  'text',
  'paragraph',
  'number',
  'date',
  'datetime',
  'select',
  'multiselect',
  'checkbox',
  'user',
  'url',
  'labels',
];

export interface CustomFieldOption {
  readonly id: string;
  readonly label: string;
  readonly color: string;
  readonly position: number;
  readonly archived: boolean;
}

/** Custom field that applies to an issue, to render its value. */
export interface IssueCustomField {
  readonly fieldId: string;
  readonly name: string;
  readonly type: CustomFieldType;
  readonly options: readonly CustomFieldOption[];
  readonly required: boolean;
}

export interface NamedRef {
  readonly id: string;
  readonly name: string;
}

export interface IssueCounts {
  readonly comments: number;
  readonly attachments: number;
  readonly children: number;
  readonly links: number;
  readonly watchers: number;
  readonly votes: number;
}

export interface IssueBreadcrumb {
  readonly project: { readonly id: string; readonly key: string; readonly name: string };
  readonly parent: {
    readonly id: string;
    readonly key: string;
    readonly summary: string;
    readonly type: string;
  } | null;
}

/** Full issue view: the issue plus what the viewer may do and the counters of its sections. */
export interface IssueView extends Issue {
  readonly startDate: string | null;
  readonly originalEstimateSeconds: number | null;
  readonly remainingEstimateSeconds: number | null;
  readonly flagged: boolean;
  readonly components: readonly NamedRef[];
  readonly fixVersions: readonly NamedRef[];
  readonly affectsVersions: readonly NamedRef[];
  readonly custom: Readonly<Record<string, unknown>>;
  readonly customFields: readonly IssueCustomField[];
  readonly permissions: readonly IssuePermission[];
  readonly availableTransitions: readonly IssueTransition[];
  readonly counts: IssueCounts;
  readonly breadcrumb: IssueBreadcrumb;
  readonly viewerHasVoted: boolean;
}

/** Partial update of an issue. Only present keys are sent; null clears the fields that allow it. */
export interface PatchIssueInput {
  readonly summary?: string;
  readonly description?: string | null;
  readonly issueTypeId?: string;
  readonly priorityId?: string;
  readonly parentId?: string | null;
  readonly assigneeId?: string | null;
  readonly storyPoints?: number | null;
  readonly originalEstimateSeconds?: number | null;
  readonly remainingEstimateSeconds?: number | null;
  readonly startDate?: string | null;
  readonly dueDate?: string | null;
  readonly flagged?: boolean;
  readonly labels?: readonly string[] | null;
  readonly components?: readonly string[] | null;
  readonly fixVersions?: readonly string[] | null;
  readonly affectsVersions?: readonly string[] | null;
  readonly custom?: Readonly<Record<string, unknown>>;
  readonly expectedUpdatedAt?: string;
}

export interface IssueChange {
  readonly field: string;
  readonly from: string | null;
  readonly to: string | null;
}

export interface PatchIssueResult {
  readonly issue: Issue;
  readonly batchId: string | null;
  readonly changes: readonly IssueChange[];
}

export type IssueLinkKindV2 =
  'blocks' | 'relates' | 'duplicates' | 'clones' | 'causes' | 'implements';

export const LINK_KIND_OPTIONS: readonly { value: IssueLinkKindV2; label: string }[] = [
  { value: 'blocks', label: 'Bloquea' },
  { value: 'relates', label: 'Se relaciona con' },
  { value: 'duplicates', label: 'Duplica' },
  { value: 'clones', label: 'Clona' },
  { value: 'causes', label: 'Causa' },
  { value: 'implements', label: 'Implementa' },
];

export interface IssueLinkEntry {
  readonly id: string;
  readonly kind: IssueLinkKindV2;
  readonly direction: 'outward' | 'inward';
  /** Label from the side of the viewed issue, for example "bloquea" or "bloqueada por". */
  readonly label: string;
  readonly issue: {
    readonly id: string;
    readonly key: string;
    readonly summary: string;
    readonly status: string;
    readonly type: string;
    readonly priority: string;
  };
}

export interface IssueChild {
  readonly id: string;
  readonly key: string;
  readonly summary: string;
  readonly type: string;
  readonly status: string;
  readonly assigneeId: string | null;
  readonly assigneeName: string | null;
}

export interface IssueChildren {
  readonly total: number;
  readonly done: number;
  readonly percent: number;
  readonly children: readonly IssueChild[];
}

export interface ProjectVersion {
  readonly id: string;
  readonly name: string;
  readonly released: boolean;
  readonly archived: boolean;
}

// Bulk, clone, move and convert.

export type BulkAction =
  'assign' | 'set_priority' | 'set_labels' | 'set_field' | 'transition' | 'move' | 'delete';

export interface BulkFailure {
  readonly issue: string;
  readonly code: string;
  readonly message: string;
}

export interface BulkResult {
  readonly succeeded: readonly Issue[];
  readonly failed: readonly BulkFailure[];
}

export type CloneInclude = 'attachments' | 'links' | 'subtasks' | 'watchers';

export interface CloneIssueInput {
  readonly summary?: string;
  readonly projectId?: string;
  readonly include?: readonly CloneInclude[];
}

export interface MoveIssueInput {
  readonly projectId: string;
  readonly issueTypeId?: string;
  readonly fieldMapping?: {
    readonly issueType?: Readonly<Record<string, string>>;
    readonly priority?: Readonly<Record<string, string>>;
    readonly status?: Readonly<Record<string, string>>;
  };
}

export interface MoveIssueResult {
  readonly issue: Issue;
  readonly warnings: readonly string[];
  readonly moved: readonly { id: string; fromKey: string; toKey: string }[];
}

export interface ConvertIssueInput {
  readonly issueTypeId: string;
  /** Undefined keeps the parent; null clears it. */
  readonly parentId?: string | null;
}
