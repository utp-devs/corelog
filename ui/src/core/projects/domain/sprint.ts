import type { TicketPriority, TicketStatus } from '@/core/tickets/domain/ticket'

export type SprintStatus = 'planned' | 'active' | 'closed'

export interface Sprint {
  id: string
  projectId: string
  name: string
  goal: string
  status: SprintStatus
  startDate?: string
  endDate?: string
  closedAt?: string
}

export interface SprintIssue {
  id: string
  title: string
  status: TicketStatus
  priority: TicketPriority
  storyPoints?: number
  closedAt?: string
}

export interface BurndownPoint {
  date: string
  ideal: number
  remaining: number
}

export interface VelocityPoint {
  sprintId: string
  name: string
  points: number
}
