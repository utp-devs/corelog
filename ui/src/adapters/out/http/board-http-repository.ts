import type { BoardCard, BoardColumn, ProjectBoard } from '@/core/projects/domain/board'
import type { TicketPriority, TicketStatus } from '@/core/tickets/domain/ticket'
import { apiClient } from './api-client'

// Forma que devuelve la API (snake_case). Se mapea a la forma de dominio (camelCase).
interface BoardCardDto {
  id: string
  type: string
  summary: string
  status: TicketStatus
  priority: TicketPriority
  assignee_id?: string
  assignee?: string
  story_points?: number
  rank: string
}

interface BoardColumnDto {
  id: string
  name: string
  status: TicketStatus
  position: number
  wip_limit?: number
  count: number
  cards: BoardCardDto[]
}

interface BoardDto {
  id: string
  project_id: string
  name: string
  type: 'kanban' | 'scrum'
  columns: BoardColumnDto[]
}

function toCard(card: BoardCardDto): BoardCard {
  return {
    id: card.id,
    type: card.type,
    summary: card.summary,
    status: card.status,
    priority: card.priority,
    assigneeId: card.assignee_id,
    assignee: card.assignee,
    storyPoints: card.story_points,
    rank: card.rank,
  }
}

function toColumn(column: BoardColumnDto): BoardColumn {
  return {
    id: column.id,
    name: column.name,
    status: column.status,
    position: column.position,
    wipLimit: column.wip_limit,
    count: column.count,
    cards: column.cards.map(toCard),
  }
}

export function toProjectBoard(board: BoardDto): ProjectBoard {
  return {
    id: board.id,
    projectId: board.project_id,
    name: board.name,
    type: board.type,
    columns: board.columns.map(toColumn),
  }
}

export class BoardHttpRepository {
  async getProjectBoard(projectId: string): Promise<ProjectBoard> {
    const { data } = await apiClient.get<{ data: BoardDto }>(`/projects/${projectId}/board`)
    return toProjectBoard(data.data)
  }

  // Mueve una incidencia a otra columna. La API devuelve el tablero actualizado.
  async moveCard(projectId: string, issueId: string, columnId: string): Promise<ProjectBoard> {
    const { data } = await apiClient.post<{ data: BoardDto }>(
      `/projects/${projectId}/board/cards/${issueId}/move`,
      { column_id: columnId },
    )
    return toProjectBoard(data.data)
  }

  // Fija el limite WIP de una columna; null lo quita. La API devuelve el tablero actualizado.
  async updateColumnWip(projectId: string, columnId: string, wipLimit: number | null): Promise<ProjectBoard> {
    const { data } = await apiClient.patch<{ data: BoardDto }>(
      `/projects/${projectId}/board/columns/${columnId}`,
      { wip_limit: wipLimit },
    )
    return toProjectBoard(data.data)
  }
}
