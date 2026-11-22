import type { BacklogItem } from '@/core/projects/domain/backlog'
import type { TicketPriority, TicketStatus } from '@/core/tickets/domain/ticket'
import { apiClient } from './api-client'

// Forma que devuelve la API (snake_case).
interface BacklogItemDto {
  id: string
  project_id: string
  summary: string
  type: string
  status: TicketStatus
  priority: TicketPriority
  rank: string
  assignee_id?: string
  story_points?: number
}

export function toBacklogItem(item: BacklogItemDto): BacklogItem {
  return {
    id: item.id,
    projectId: item.project_id,
    summary: item.summary,
    type: item.type,
    status: item.status,
    priority: item.priority,
    rank: item.rank,
    assigneeId: item.assignee_id,
    storyPoints: item.story_points,
  }
}

export class BacklogHttpRepository {
  async list(projectId: string): Promise<BacklogItem[]> {
    const { data } = await apiClient.get<{ data: BacklogItemDto[] }>('/backlog', {
      params: { project_id: projectId },
    })
    return data.data.map(toBacklogItem)
  }

  // Coloca la incidencia en la posicion indicada (0 es la primera). Devuelve el backlog nuevo.
  async move(projectId: string, issueId: string, position: number): Promise<BacklogItem[]> {
    const { data } = await apiClient.put<{ data: BacklogItemDto[] }>(
      `/backlog/${issueId}`,
      { position },
      { params: { project_id: projectId } },
    )
    return data.data.map(toBacklogItem)
  }
}
