import type {
  CreateTicketInput,
  ListTicketsFilter,
  TicketPage,
  TicketRepositoryPort,
  UpdateIssueMetadataInput,
  UpdateIssueDetailsInput,
  IssueLinkKind,
} from './ports/ticket-repository.port'
import type { Ticket, TicketStatus } from '../domain/ticket'

export class TicketService {
  private readonly repository: TicketRepositoryPort

  constructor(repository: TicketRepositoryPort) {
    this.repository = repository
  }

  list(filter: ListTicketsFilter = {}): Promise<TicketPage> {
    return this.repository.list(filter)
  }

  getById(id: string): Promise<Ticket> {
    return this.repository.getById(id)
  }

  create(input: CreateTicketInput): Promise<Ticket> {
    return this.repository.create(input)
  }

  updateStatus(id: string, status: TicketStatus): Promise<Ticket> {
    return this.repository.updateStatus(id, status)
  }

  assign(id: string, assigneeId: string | null): Promise<Ticket> {
    return this.repository.assign(id, assigneeId)
  }

  updateMetadata(id: string, input: UpdateIssueMetadataInput): Promise<Ticket> {
    return this.repository.updateMetadata(id, input)
  }

  updateDetails(id: string, input: UpdateIssueDetailsInput): Promise<Ticket> {
    return this.repository.updateDetails(id, input)
  }

  listComments(id: string) {
    return this.repository.listComments(id)
  }

  createComment(id: string, body: string) {
    return this.repository.createComment(id, body)
  }

  deleteComment(issueId: string, commentId: string) {
    return this.repository.deleteComment(issueId, commentId)
  }

  listActivities(id: string) {
    return this.repository.listActivities(id)
  }

  listLinks(id: string) {
    return this.repository.listLinks(id)
  }

  createLink(id: string, targetIssueId: string, kind: IssueLinkKind) {
    return this.repository.createLink(id, targetIssueId, kind)
  }

  deleteLink(id: string, linkId: string) {
    return this.repository.deleteLink(id, linkId)
  }

  updateRank(id: string, rank: string): Promise<Ticket> {
    return this.repository.updateRank(id, rank)
  }
}
