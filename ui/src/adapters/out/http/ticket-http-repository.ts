import type {
  CreateTicketInput,
  ListTicketsFilter,
  TicketPage,
  TicketRepositoryPort,
  UpdateIssueMetadataInput,
  UpdateIssueDetailsInput,
  IssueComment,
  IssueActivity,
  IssueLink,
  IssueLinkKind,
} from '@/core/tickets/application/ports/ticket-repository.port'
import type { IssueType, Ticket, TicketPriority, TicketStatus } from '@/core/tickets/domain/ticket'
import { apiClient } from './api-client'

interface TicketDto {
  id: string
  project_id: string
  title: string
  description: string
  status: TicketStatus
  priority: TicketPriority
  issue_type: IssueType
  parent_id: string | null
  due_date: string | null
  story_points: number | null
  rank: string
  closed_at: string | null
  requester_id: string
  requester_name: string
  assignee_id: string | null
  assignee_name?: string | null
  created_at: string
  updated_at: string
}

interface CommentDto {
  id: string
  issue_id: string
  author_id: string
  body: string
  created_at: string
  updated_at: string
  deleted_at: string | null
}

interface ActivityDto {
  id: string
  issue_id: string
  actor_id: string
  action: string
  field_name: string | null
  old_value: string | null
  new_value: string | null
  created_at: string
}

interface LinkDto {
  id: string
  source_issue_id: string
  target_issue_id: string
  kind: IssueLinkKind
  created_by: string
  created_at: string
}

function toComment(dto: CommentDto): IssueComment {
  return {
    id: dto.id,
    issueId: dto.issue_id,
    authorId: dto.author_id,
    body: dto.body,
    createdAt: dto.created_at,
    updatedAt: dto.updated_at,
    deletedAt: dto.deleted_at,
  }
}

function toActivity(dto: ActivityDto): IssueActivity {
  return {
    id: dto.id,
    issueId: dto.issue_id,
    actorId: dto.actor_id,
    action: dto.action,
    fieldName: dto.field_name,
    oldValue: dto.old_value,
    newValue: dto.new_value,
    createdAt: dto.created_at,
  }
}

function toLink(dto: LinkDto): IssueLink {
  return {
    id: dto.id,
    sourceIssueId: dto.source_issue_id,
    targetIssueId: dto.target_issue_id,
    kind: dto.kind,
    createdBy: dto.created_by,
    createdAt: dto.created_at,
  }
}

function toTicket(dto: TicketDto): Ticket {
  return {
    id: dto.id,
    projectId: dto.project_id,
    title: dto.title,
    description: dto.description,
    status: dto.status,
    priority: dto.priority,
    issueType: dto.issue_type,
    parentId: dto.parent_id,
    dueDate: dto.due_date,
    storyPoints: dto.story_points,
    rank: dto.rank,
    closedAt: dto.closed_at,
    requesterId: dto.requester_id,
    requesterName: dto.requester_name,
    assigneeId: dto.assignee_id ?? null,
    assigneeName: dto.assignee_name ?? null,
    createdAt: dto.created_at,
    updatedAt: dto.updated_at,
  }
}

export class TicketHttpRepository implements TicketRepositoryPort {
  async list(filter: ListTicketsFilter): Promise<TicketPage> {
    const { data } = await apiClient.get<{ data: TicketDto[]; meta: { page: number; page_size: number; total: number } }>('/issues', {
      params: {
        status: filter.status,
        priority: filter.priority,
        q: filter.query,
        issue_type: filter.issueType,
        label_id: filter.labelId,
        assignee_id: filter.assigneeId,
        due_from: filter.dueFrom,
        due_to: filter.dueTo,
        page: filter.page,
        page_size: filter.pageSize,
      },
    })
    return {
      items: data.data.map(toTicket),
      total: data.meta.total,
      page: data.meta.page,
      pageSize: data.meta.page_size,
    }
  }

  async getById(id: string): Promise<Ticket> {
    const { data } = await apiClient.get<{ data: TicketDto }>(`/issues/${id}`)
    return toTicket(data.data)
  }

  async create(input: CreateTicketInput): Promise<Ticket> {
    const { data } = await apiClient.post<{ data: TicketDto }>('/issues', {
      title: input.title,
      description: input.description,
      priority: input.priority,
      issue_type: input.issueType,
      parent_id: input.parentId,
      due_date: input.dueDate,
      story_points: input.storyPoints,
    })
    return toTicket(data.data)
  }

  async updateStatus(id: string, status: TicketStatus): Promise<Ticket> {
    const { data } = await apiClient.patch<{ data: TicketDto }>(`/issues/${id}/status`, { status })
    return toTicket(data.data)
  }

  async assign(id: string, assigneeId: string): Promise<Ticket> {
    const { data } = await apiClient.patch<{ data: TicketDto }>(`/issues/${id}/assign`, {
      assignee_id: assigneeId,
    })
    return toTicket(data.data)
  }

  async updateMetadata(id: string, input: UpdateIssueMetadataInput): Promise<Ticket> {
    const { data } = await apiClient.patch<{ data: TicketDto }>(`/issues/${id}/metadata`, {
      issue_type: input.issueType,
      parent_id: input.parentId,
      due_date: input.dueDate,
      story_points: input.storyPoints,
    })
    return toTicket(data.data)
  }

  async updateDetails(id: string, input: UpdateIssueDetailsInput): Promise<Ticket> {
    const { data } = await apiClient.patch<{ data: TicketDto }>(`/issues/${id}/details`, {
      title: input.title,
      description: input.description,
      priority: input.priority,
      issue_type: input.issueType,
      parent_id: input.parentId,
      due_date: input.dueDate,
      story_points: input.storyPoints,
    })
    return toTicket(data.data)
  }

  async listComments(id: string): Promise<IssueComment[]> {
    const { data } = await apiClient.get<{ data: CommentDto[] }>(`/issues/${id}/comments`)
    return data.data.map(toComment)
  }

  async createComment(id: string, body: string): Promise<IssueComment> {
    const { data } = await apiClient.post<{ data: CommentDto }>(`/issues/${id}/comments`, { body })
    return toComment(data.data)
  }

  async deleteComment(issueId: string, commentId: string): Promise<void> {
    await apiClient.delete(`/issues/${issueId}/comments/${commentId}`)
  }

  async listActivities(id: string): Promise<IssueActivity[]> {
    const { data } = await apiClient.get<{ data: ActivityDto[] }>(`/issues/${id}/activities`)
    return data.data.map(toActivity)
  }

  async listLinks(id: string): Promise<IssueLink[]> {
    const { data } = await apiClient.get<{ data: LinkDto[] }>(`/issues/${id}/links`)
    return data.data.map(toLink)
  }

  async createLink(id: string, targetIssueId: string, kind: IssueLinkKind): Promise<IssueLink> {
    const { data } = await apiClient.post<{ data: LinkDto }>(`/issues/${id}/links`, {
      target_issue_id: targetIssueId,
      kind,
    })
    return toLink(data.data)
  }

  async deleteLink(id: string, linkId: string): Promise<void> {
    await apiClient.delete(`/issues/${id}/links/${linkId}`)
  }

  async updateRank(id: string, rank: string): Promise<Ticket> {
    const { data } = await apiClient.patch<{ data: TicketDto }>(`/issues/${id}/rank`, { rank })
    return toTicket(data.data)
  }
}
