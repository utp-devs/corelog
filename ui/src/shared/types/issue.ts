/**
 * Issue vocabulary shared by the design system and the domain. Lives in shared so that shared components
 * do not depend on core; core/tickets re-exports these names.
 */
export type IssueStatusKind = 'open' | 'in_progress' | 'resolved' | 'closed'
export type IssuePriorityKind = 'low' | 'medium' | 'high' | 'critical'
export type IssueTypeKind = 'task' | 'bug' | 'story' | 'epic' | 'subtask'
