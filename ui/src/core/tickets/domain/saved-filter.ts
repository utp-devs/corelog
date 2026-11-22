import type { IssueType, TicketPriority, TicketStatus } from './ticket'

export interface SavedFilterCriteria {
  status?: TicketStatus
  priority?: TicketPriority
  query?: string
  issueType?: IssueType
  dueFrom?: string
  dueTo?: string
}

export interface SavedFilter {
  id: string
  name: string
  criteria: SavedFilterCriteria
  createdAt: string
  updatedAt: string
}
