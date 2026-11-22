import { afterEach, describe, expect, it, vi } from 'vitest'

import { apiClient } from './api-client'
import { BoardHttpRepository } from './board-http-repository'

const boardDto = {
  id: 'board-1',
  project_id: 'project-1',
  name: 'Default',
  type: 'kanban' as const,
  columns: [{
    id: 'column-1',
    name: 'En curso',
    status: 'in_progress' as const,
    position: 1,
    wip_limit: 5,
    count: 1,
    cards: [{
      id: 'issue-1',
      type: 'bug',
      summary: 'Error al exportar',
      status: 'in_progress' as const,
      priority: 'high' as const,
      assignee_id: 'user-1',
      assignee: 'Ada Lovelace',
      story_points: 3,
      rank: 'm',
    }],
  }],
}

describe('BoardHttpRepository', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('maps the board from snake_case to the domain shape', async () => {
    vi.spyOn(apiClient, 'get').mockResolvedValue({ data: { data: boardDto } })

    const board = await new BoardHttpRepository().getProjectBoard('project-1')

    expect(board.projectId).toBe('project-1')
    const column = board.columns[0]
    expect(column.wipLimit).toBe(5)
    expect(column.cards[0].storyPoints).toBe(3)
    expect(column.cards[0].assigneeId).toBe('user-1')
  })

  it('sends the move request with column_id and returns the updated board', async () => {
    const post = vi.spyOn(apiClient, 'post').mockResolvedValue({ data: { data: boardDto } })

    const board = await new BoardHttpRepository().moveCard('project-1', 'issue-1', 'column-1')

    expect(post).toHaveBeenCalledWith('/projects/project-1/board/cards/issue-1/move', { column_id: 'column-1' })
    expect(board.columns[0].cards).toHaveLength(1)
  })

  it('sends null to clear a WIP limit', async () => {
    const patch = vi.spyOn(apiClient, 'patch').mockResolvedValue({ data: { data: boardDto } })

    await new BoardHttpRepository().updateColumnWip('project-1', 'column-1', null)

    expect(patch).toHaveBeenCalledWith('/projects/project-1/board/columns/column-1', { wip_limit: null })
  })
})
