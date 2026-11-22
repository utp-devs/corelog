import { afterEach, describe, expect, it, vi } from 'vitest'

import { apiClient } from './api-client'
import { BacklogHttpRepository } from './backlog-http-repository'

const itemDto = {
  id: 'issue-1',
  project_id: 'project-1',
  summary: 'Revisar exportacion',
  type: 'bug',
  status: 'open' as const,
  priority: 'high' as const,
  rank: '000001',
  assignee_id: 'user-1',
  story_points: 2,
}

describe('BacklogHttpRepository', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('lists the backlog for a project and maps the items', async () => {
    const get = vi.spyOn(apiClient, 'get').mockResolvedValue({ data: { data: [itemDto] } })

    const items = await new BacklogHttpRepository().list('project-1')

    expect(get).toHaveBeenCalledWith('/backlog', { params: { project_id: 'project-1' } })
    expect(items[0]).toMatchObject({ id: 'issue-1', projectId: 'project-1', summary: 'Revisar exportacion', storyPoints: 2, assigneeId: 'user-1' })
  })

  it('sends the target position and returns the reordered backlog', async () => {
    const put = vi.spyOn(apiClient, 'put').mockResolvedValue({ data: { data: [itemDto] } })

    const items = await new BacklogHttpRepository().move('project-1', 'issue-1', 0)

    expect(put).toHaveBeenCalledWith('/backlog/issue-1', { position: 0 }, { params: { project_id: 'project-1' } })
    expect(items).toHaveLength(1)
  })
})
