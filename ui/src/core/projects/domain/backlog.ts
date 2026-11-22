import type { TicketPriority, TicketStatus } from '@/core/tickets/domain/ticket'

export interface BacklogItem {
  id: string
  projectId: string
  summary: string
  type: string
  status: TicketStatus
  priority: TicketPriority
  rank: string
  assigneeId?: string
  storyPoints?: number
}
