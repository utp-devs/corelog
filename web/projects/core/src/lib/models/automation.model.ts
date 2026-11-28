export type AutomationTriggerType =
  | 'issue_created'
  | 'issue_updated'
  | 'issue_transitioned'
  | 'comment_added'
  | 'sprint_started'
  | 'sprint_completed'
  | 'scheduled';

export type AutomationConditionType =
  'query' | 'field_equals' | 'field_in' | 'actor_role' | 'issue_type';

export type AutomationActionType =
  | 'transition_issue'
  | 'assign_issue'
  | 'set_field'
  | 'add_comment'
  | 'add_label'
  | 'remove_label'
  | 'create_subtask'
  | 'send_webhook'
  | 'send_notification';

/** "When" part of a rule. Only the fields of its type are meaningful. */
export interface AutomationTrigger {
  readonly type: AutomationTriggerType;
  readonly field?: string;
  readonly from?: string;
  readonly to?: string;
  readonly everyMinutes?: number;
  readonly query?: string;
}

export interface AutomationCondition {
  readonly type: AutomationConditionType;
  readonly query?: string;
  readonly field?: string;
  readonly value?: string;
  readonly values?: readonly string[];
  readonly role?: string;
}

export interface AutomationAction {
  readonly type: AutomationActionType;
  readonly status?: string;
  readonly who?: string;
  readonly userId?: string;
  readonly teamId?: string;
  readonly field?: string;
  readonly value?: string;
  readonly text?: string;
  readonly label?: string;
  readonly url?: string;
}

/** What is sent to create or change a rule. */
export interface AutomationRuleDraft {
  readonly name: string;
  readonly enabled: boolean;
  readonly trigger: AutomationTrigger;
  readonly conditions: readonly AutomationCondition[];
  readonly actions: readonly AutomationAction[];
}

export interface AutomationRule extends AutomationRuleDraft {
  readonly id: string;
  readonly projectId: string | null;
  readonly version: number;
  readonly runCount: number;
  readonly lastRunAt: string | null;
  readonly createdAt: string;
  readonly updatedAt: string;
}

export type AutomationRunStatus = 'success' | 'skipped' | 'failed';

export interface AutomationRun {
  readonly id: string;
  readonly ruleId: string;
  readonly issueId: string | null;
  readonly status: AutomationRunStatus;
  readonly detail: string;
  readonly createdAt: string;
}

export interface AutomationValidation {
  readonly valid: boolean;
  readonly errors: readonly string[];
  readonly plannedActions: readonly string[];
}
