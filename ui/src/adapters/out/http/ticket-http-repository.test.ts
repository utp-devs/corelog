import { afterEach, describe, expect, it, vi } from 'vitest'

import { apiClient } from './api-client'
import { TicketHttpRepository } from './ticket-http-repository'

const issue = {
  id: '11111111-1111-4111-8111-111111111111',
  project_id: '00000000-0000-4000-8000-000000000001',
  title: 'Error al exportar',
  description: 'Detalle',
  status: 'open',
  priority: 'high',
  issue_type: 'bug',
  parent_id: null,
  due_date: null,
  story_points: null,
  rank: 'm',
  closed_at: null,
  requester_id: '22222222-2222-4222-8222-222222222222',
  assignee_id: null,
  created_at: '2026-11-22T00:00:00Z',
  updated_at: '2026-11-22T00:00:00Z',
}

describe('TicketHttpRepository.list', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('maps the page items and the pagination meta to camelCase', async () => {
    vi.spyOn(apiClient, 'get').mockResolvedValue({
      data: { data: [issue], meta: { page: 2, page_size: 20, total: 41 } },
    })

    const page = await new TicketHttpRepository().list({ page: 2, pageSize: 20 })

    expect(page.total).toBe(41)
    expect(page.page).toBe(2)
    expect(page.pageSize).toBe(20)
    expect(page.items).toHaveLength(1)
    expect(page.items[0]).toMatchObject({ id: issue.id, title: 'Error al exportar', status: 'open' })
  })

  it('sends the pagination parameters to the issues endpoint', async () => {
    const get = vi.spyOn(apiClient, 'get').mockResolvedValue({
      data: { data: [], meta: { page: 3, page_size: 10, total: 0 } },
    })

    await new TicketHttpRepository().list({ page: 3, pageSize: 10 })

    expect(get).toHaveBeenCalledWith('/issues', expect.objectContaining({
      params: expect.objectContaining({ page: 3, page_size: 10 }),
    }))
  })
})
