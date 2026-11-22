import { afterEach, describe, expect, it, vi } from 'vitest'

import { apiClient } from './api-client'
import { SprintHttpRepository } from './sprint-http-repository'

const sprintDto = {
  id: 'sprint-1',
  project_id: 'project-1',
  name: 'Sprint 1',
  goal: 'Entregar exportacion',
  status: 'active' as const,
  start_date: '2026-11-02',
  end_date: '2026-11-08',
  closed_at: null,
}

describe('SprintHttpRepository', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('maps sprints from snake_case and drops null dates', async () => {
    vi.spyOn(apiClient, 'get').mockResolvedValue({ data: { data: [sprintDto] } })

    const [sprint] = await new SprintHttpRepository().list('project-1')

    expect(sprint).toMatchObject({ id: 'sprint-1', projectId: 'project-1', status: 'active', startDate: '2026-11-02', endDate: '2026-11-08' })
    expect(sprint.closedAt).toBeUndefined()
  })

  it('sends the close option and the target sprint', async () => {
    const post = vi.spyOn(apiClient, 'post').mockResolvedValue({ data: { data: { ...sprintDto, status: 'closed' } } })

    await new SprintHttpRepository().close('project-1', 'sprint-1', 'sprint', 'sprint-2')

    expect(post).toHaveBeenCalledWith(
      '/sprints/sprint-1/close',
      { unfinished: 'sprint', target_sprint_id: 'sprint-2' },
      { params: { project_id: 'project-1' } },
    )
  })

  it('sends null dates when the sprint is created without them', async () => {
    const post = vi.spyOn(apiClient, 'post').mockResolvedValue({ data: { data: sprintDto } })

    await new SprintHttpRepository().create('project-1', { name: 'Sprint 1', goal: '' })

    expect(post).toHaveBeenCalledWith(
      '/sprints',
      { name: 'Sprint 1', goal: '', start_date: null, end_date: null },
      { params: { project_id: 'project-1' } },
    )
  })

  it('maps velocity points', async () => {
    vi.spyOn(apiClient, 'get').mockResolvedValue({ data: { data: [{ sprint_id: 'sprint-0', name: 'Sprint 0', points: 13 }] } })

    const points = await new SprintHttpRepository().velocity('project-1')

    expect(points).toEqual([{ sprintId: 'sprint-0', name: 'Sprint 0', points: 13 }])
  })
})
