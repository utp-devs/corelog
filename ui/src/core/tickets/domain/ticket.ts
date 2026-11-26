import type { IssuePriorityKind, IssueStatusKind, IssueTypeKind } from '@/shared/types/issue'

export type TicketStatus = IssueStatusKind
export type TicketPriority = IssuePriorityKind
export type IssueType = IssueTypeKind

export interface Ticket {
  id: string
  title: string
  description: string
  status: TicketStatus
  priority: TicketPriority
  projectId: string
  issueType: IssueType
  parentId: string | null
  dueDate: string | null
  storyPoints: number | null
  rank: string
  closedAt: string | null
  requesterId: string
  requesterName: string
  assigneeId: string | null
  assigneeName: string | null
  createdAt: string
  updatedAt: string
}

const allowedTransitions: Record<TicketStatus, TicketStatus[]> = {
  open: ['in_progress', 'closed'],
  in_progress: ['resolved', 'closed'],
  resolved: ['closed', 'in_progress'],
  closed: [],
}

export function canTransitionTo(current: TicketStatus, next: TicketStatus): boolean {
  return allowedTransitions[current].includes(next)
}
