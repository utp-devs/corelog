import { afterEach, describe, expect, it, vi } from 'vitest'

import { apiClient } from './api-client'
import { CalendarHttpRepository } from './calendar-http-repository'

describe('CalendarHttpRepository', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('requests the range with dates and maps the events', async () => {
    const get = vi.spyOn(apiClient, 'get').mockResolvedValue({
      data: { data: [{ kind: 'issue_due', date: '2026-11-05', title: 'Entregar', source_id: 'issue-1', link: '/issues/issue-1' }] },
    })

    const events = await new CalendarHttpRepository().events('project-1', '2026-11-01', '2026-11-30')

    expect(get).toHaveBeenCalledWith('/calendar/events', { params: { project_id: 'project-1', from: '2026-11-01', to: '2026-11-30' } })
    expect(events).toEqual([{ kind: 'issue_due', date: '2026-11-05', title: 'Entregar', sourceId: 'issue-1', link: '/issues/issue-1' }])
  })
})
