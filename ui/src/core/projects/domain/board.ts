import type { TicketPriority, TicketStatus } from '@/core/tickets/domain/ticket'

export interface BoardCard {
  id: string
  type: string
  summary: string
  status: TicketStatus
  priority: TicketPriority
  assigneeId?: string
  assignee?: string
  storyPoints?: number
  rank: string
}

export interface BoardColumn {
  id: string
  name: string
  status: TicketStatus
  position: number
  wipLimit?: number
  count: number
  cards: BoardCard[]
}

export interface ProjectBoard {
  id: string
  projectId: string
  name: string
  type: 'kanban' | 'scrum'
  columns: BoardColumn[]
  canManageBoard: boolean
}
