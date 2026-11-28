/** Events a notification scheme can route. */
export const NOTIFICATION_EVENTS = [
  'issue_created',
  'issue_assigned',
  'issue_updated',
  'issue_transitioned',
  'issue_resolved',
  'issue_commented',
  'issue_mentioned',
  'comment_edited',
  'worklog_logged',
  'version_released',
  'sprint_started',
  'sprint_completed',
  'filter_subscription',
] as const;

export type NotificationEvent = (typeof NOTIFICATION_EVENTS)[number];

export type NotificationChannel = 'in_app' | 'email';

export const NOTIFICATION_CHANNELS: readonly NotificationChannel[] = ['in_app', 'email'];

export type NotificationRecipientType =
  | 'reporter'
  | 'assignee'
  | 'watchers'
  | 'project_lead'
  | 'component_lead'
  | 'mentioned'
  | 'reply_target'
  | 'user'
  | 'team'
  | 'role';

/** A person, team or role-based recipient. `id` only for user and team; `role` only for role. */
export interface NotificationRecipient {
  readonly type: NotificationRecipientType;
  readonly id?: string;
  readonly role?: string;
}

export interface NotificationRule {
  readonly event: NotificationEvent;
  readonly recipients: readonly NotificationRecipient[];
  readonly channels: readonly NotificationChannel[];
  /** True when the rule belongs to this scope; false when it is inherited. */
  readonly overridden: boolean;
}

export type NotificationSchemeScope =
  | { readonly kind: 'project'; readonly id: string }
  | { readonly kind: 'organization'; readonly id: string };

export interface NotificationScheme {
  readonly scope: 'project' | 'organization';
  readonly organizationId: string | null;
  readonly projectId: string | null;
  readonly isDefault: boolean;
  readonly updatedAt: string | null;
  readonly rules: readonly NotificationRule[];
}

/** Rule sent to the API (the `overridden` flag is never sent). */
export interface NotificationRuleInput {
  readonly event: NotificationEvent;
  readonly recipients: readonly NotificationRecipient[];
  readonly channels: readonly NotificationChannel[];
}
