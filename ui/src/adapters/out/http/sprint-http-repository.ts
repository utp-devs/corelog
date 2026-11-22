import type { BurndownPoint, Sprint, SprintIssue, SprintStatus, VelocityPoint } from '@/core/projects/domain/sprint'
import type { TicketPriority, TicketStatus } from '@/core/tickets/domain/ticket'
import { apiClient } from './api-client'

// Formas que devuelve la API (snake_case). Se mapean al dominio (camelCase).
interface SprintDto {
  id: string
  project_id: string
  name: string
  goal: string
  status: SprintStatus
  start_date?: string | null
  end_date?: string | null
  closed_at?: string | null
}

interface SprintIssueDto {
  id: string
  title: string
  status: TicketStatus
  priority: TicketPriority
  story_points?: number | null
  closed_at?: string | null
}

interface BurndownDto {
  date: string
  ideal: number
  remaining: number
}

interface VelocityDto {
  sprint_id: string
  name: string
  points: number
}

function toSprint(dto: SprintDto): Sprint {
  return {
    id: dto.id,
    projectId: dto.project_id,
    name: dto.name,
    goal: dto.goal,
    status: dto.status,
    startDate: dto.start_date ?? undefined,
    endDate: dto.end_date ?? undefined,
    closedAt: dto.closed_at ?? undefined,
  }
}

function toSprintIssue(dto: SprintIssueDto): SprintIssue {
  return {
    id: dto.id,
    title: dto.title,
    status: dto.status,
    priority: dto.priority,
    storyPoints: dto.story_points ?? undefined,
    closedAt: dto.closed_at ?? undefined,
  }
}

export interface CreateSprintInput {
  name: string
  goal: string
  startDate?: string
  endDate?: string
}

export class SprintHttpRepository {
  async list(projectId: string): Promise<Sprint[]> {
    const { data } = await apiClient.get<{ data: SprintDto[] }>('/sprints', { params: { project_id: projectId } })
    return data.data.map(toSprint)
  }

  async create(projectId: string, input: CreateSprintInput): Promise<Sprint> {
    const { data } = await apiClient.post<{ data: SprintDto }>(
      '/sprints',
      {
        name: input.name,
        goal: input.goal,
        start_date: input.startDate || null,
        end_date: input.endDate || null,
      },
      { params: { project_id: projectId } },
    )
    return toSprint(data.data)
  }

  async start(projectId: string, sprintId: string): Promise<Sprint> {
    const { data } = await apiClient.post<{ data: SprintDto }>(
      `/sprints/${sprintId}/start`,
      {},
      { params: { project_id: projectId } },
    )
    return toSprint(data.data)
  }

  // unfinished: 'backlog' quita las incidencias no cerradas; 'sprint' las mueve al sprint destino.
  async close(projectId: string, sprintId: string, unfinished: 'backlog' | 'sprint', targetSprintId?: string): Promise<Sprint> {
    const { data } = await apiClient.post<{ data: SprintDto }>(
      `/sprints/${sprintId}/close`,
      { unfinished, target_sprint_id: targetSprintId ?? null },
      { params: { project_id: projectId } },
    )
    return toSprint(data.data)
  }

  async listIssues(projectId: string, sprintId: string): Promise<SprintIssue[]> {
    const { data } = await apiClient.get<{ data: SprintIssueDto[] }>(`/sprints/${sprintId}/issues`, {
      params: { project_id: projectId },
    })
    return data.data.map(toSprintIssue)
  }

  async assignIssue(projectId: string, sprintId: string, issueId: string): Promise<void> {
    await apiClient.put(`/sprints/${sprintId}/issues/${issueId}`, {}, { params: { project_id: projectId } })
  }

  async removeIssue(projectId: string, sprintId: string, issueId: string): Promise<void> {
    await apiClient.delete(`/sprints/${sprintId}/issues/${issueId}`, { params: { project_id: projectId } })
  }

  async burndown(projectId: string, sprintId: string): Promise<BurndownPoint[]> {
    const { data } = await apiClient.get<{ data: BurndownDto[] }>(`/sprints/${sprintId}/burndown`, {
      params: { project_id: projectId },
    })
    return data.data
  }

  async velocity(projectId: string, limit = 5): Promise<VelocityPoint[]> {
    const { data } = await apiClient.get<{ data: VelocityDto[] }>('/sprints/velocity', {
      params: { project_id: projectId, limit },
    })
    return data.data.map((point) => ({ sprintId: point.sprint_id, name: point.name, points: point.points }))
  }
}
