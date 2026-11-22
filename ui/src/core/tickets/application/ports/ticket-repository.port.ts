import type { IssueType, Ticket, TicketPriority, TicketStatus } from '@/core/tickets/domain/ticket'

export interface CreateTicketInput {
  title: string
  description: string
  priority: TicketPriority
  issueType?: IssueType
  parentId?: string
  dueDate?: string
  storyPoints?: number
}

export interface UpdateIssueMetadataInput {
  issueType: IssueType
  parentId?: string
  dueDate?: string
  storyPoints?: number
}

export interface ListTicketsFilter {
  status?: TicketStatus
  priority?: TicketPriority
  page?: number
  pageSize?: number
}

export interface TicketRepositoryPort {
  list(filter: ListTicketsFilter): Promise<Ticket[]>
  getById(id: string): Promise<Ticket>
  create(input: CreateTicketInput): Promise<Ticket>
  updateStatus(id: string, status: TicketStatus): Promise<Ticket>
  assign(id: string, assigneeId: string): Promise<Ticket>
  updateMetadata(id: string, input: UpdateIssueMetadataInput): Promise<Ticket>
  listComments(id: string): Promise<IssueComment[]>
  createComment(id: string, body: string): Promise<IssueComment>
  deleteComment(issueId: string, commentId: string): Promise<void>
  listActivities(id: string): Promise<IssueActivity[]>
  listLinks(id: string): Promise<IssueLink[]>
  createLink(id: string, targetIssueId: string, kind: IssueLinkKind): Promise<IssueLink>
  deleteLink(id: string, linkId: string): Promise<void>
}

export interface IssueComment {
  id: string
  issueId: string
  authorId: string
  body: string
  createdAt: string
  updatedAt: string
  deletedAt: string | null
}

export interface IssueActivity {
  id: string
  issueId: string
  actorId: string
  action: string
  fieldName: string | null
  oldValue: string | null
  newValue: string | null
  createdAt: string
}

export type IssueLinkKind = 'blocks' | 'relates' | 'duplicates'

export interface IssueLink {
  id: string
  sourceIssueId: string
  targetIssueId: string
  kind: IssueLinkKind
  createdBy: string
  createdAt: string
}
