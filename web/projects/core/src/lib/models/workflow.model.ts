import type { Issue } from './issue.model';

/** Category of a workflow status; it decides the color and the legacy status mirror. */
export type WorkflowCategory = 'todo' | 'in_progress' | 'done';

export const WORKFLOW_CATEGORIES: readonly WorkflowCategory[] = ['todo', 'in_progress', 'done'];

export const WORKFLOW_CATEGORY_LABELS: Readonly<Record<WorkflowCategory, string>> = {
  todo: 'Por hacer',
  in_progress: 'En curso',
  done: 'Hecho',
};

/** Status as embedded in an issue or a transition. */
export interface WorkflowStatusRef {
  readonly id: string;
  readonly name: string;
  readonly category: WorkflowCategory;
}

/** Status of the organization catalog. */
export interface OrganizationStatus {
  readonly id: string;
  readonly name: string;
  readonly category: WorkflowCategory;
  readonly position: number;
}

export interface OrganizationStatusInput {
  readonly name: string;
  readonly category: WorkflowCategory;
}

export interface OrganizationStatusPatch {
  readonly name?: string;
  readonly category?: WorkflowCategory;
}

/** Condition, validator or post-function: a typed tag and its parameters. */
export interface WorkflowRule {
  readonly type: string;
  readonly params: Readonly<Record<string, unknown>>;
}

/** Transition of an editable workflow. `from` and `to` hold status refs (ids for saved statuses). */
export interface WorkflowTransition {
  readonly id: string;
  readonly name: string;
  /** Empty means "from any status". */
  readonly from: readonly string[];
  readonly to: string;
  readonly conditions: readonly WorkflowRule[];
  readonly validators: readonly WorkflowRule[];
  readonly postFunctions: readonly WorkflowRule[];
  readonly requiredFields: readonly string[];
}

/** Status of a project workflow, with how many issues of the project sit in it. */
export interface WorkflowStatus extends WorkflowStatusRef {
  readonly position: number;
  readonly issueCount: number;
}

/** Workflow of a project as the API returns it. */
export interface WorkflowView {
  readonly statuses: readonly WorkflowStatus[];
  readonly initialStatusId: string;
  readonly transitions: readonly WorkflowTransition[];
  readonly version: number;
  readonly updatedAt: string;
  readonly isTemplateDefault: boolean;
}

/** Problem found while validating a workflow definition. `path` looks like `transitions[3].to`. */
export interface WorkflowIssue {
  readonly path: string;
  readonly code: string;
  readonly message: string;
}

export interface WorkflowMappingRequirement {
  readonly statusId: string;
  readonly name: string;
  readonly issueCount: number;
}

export interface WorkflowValidation {
  readonly valid: boolean;
  readonly errors: readonly WorkflowIssue[];
  readonly warnings: readonly WorkflowIssue[];
  readonly mappingRequired: readonly WorkflowMappingRequirement[];
}

export interface WorkflowSaveResult {
  readonly workflow: WorkflowView;
  readonly warnings: readonly WorkflowIssue[];
}

/** Status of the editable definition. Saved statuses carry `id`; new ones only `name` and `category`. */
export interface WorkflowStatusInput {
  readonly ref: string;
  readonly id?: string;
  readonly name?: string;
  readonly category?: WorkflowCategory;
}

export interface WorkflowDefinitionInput {
  readonly expectedVersion: number;
  readonly initialRef: string;
  readonly statuses: readonly WorkflowStatusInput[];
  readonly transitions: readonly WorkflowTransition[];
  /** Removed status id -> status id that receives its issues. */
  readonly statusMapping: Readonly<Record<string, string>>;
}

export type WorkflowTemplateId = 'scrum' | 'kanban' | 'task_tracking' | 'bug_tracking';

export interface WorkflowTemplate {
  readonly id: WorkflowTemplateId;
  readonly name: string;
  readonly description: string;
  readonly initialRef: string;
  readonly statuses: readonly {
    readonly ref: string;
    readonly name: string;
    readonly category: WorkflowCategory;
  }[];
  readonly transitions: readonly WorkflowTransition[];
}

export interface WorkflowMigrationResult {
  readonly moved: number;
}

/** Transition an actor may run on an issue now, with what the client must ask first. */
export interface IssueTransition {
  readonly id: string;
  readonly name: string;
  readonly toStatus: string;
  readonly to: WorkflowStatusRef;
  readonly requiredFields: readonly string[];
  readonly requiresComment: boolean;
  readonly requiresResolution: boolean;
  readonly requiresAssignee: boolean;
  readonly needsScreen: boolean;
}

export interface RunTransitionInput {
  readonly transitionId: string;
  readonly fields?: Readonly<Record<string, string>>;
  readonly comment?: string;
  readonly resolutionId?: string;
  readonly assigneeId?: string;
  readonly expectedStatusId?: string;
}

/** Transition applied by the server, returned with the updated issue. */
export interface AppliedTransition {
  readonly id: string;
  readonly name: string;
  readonly fromStatusId: string;
  readonly to: WorkflowStatusRef;
}

/** Per-issue outcome of a bulk transition. */
export interface BulkTransitionResult {
  readonly succeeded: readonly Issue[];
  readonly failed: readonly {
    readonly issue: string;
    readonly code: string;
    readonly message: string;
  }[];
}
