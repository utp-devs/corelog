import { afterEach, describe, expect, it, vi } from 'vitest'

import { apiClient } from './api-client'
import { NotificationHttpRepository } from './notification-http-repository'

describe('NotificationHttpRepository', () => {
  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('maps notifications from snake_case', async () => {
    vi.spyOn(apiClient, 'get').mockResolvedValue({
      data: { data: [{ id: 'n-1', kind: 'issue_assigned', message: 'Te asignaron', link: '/issues/1', read: false, created_at: '2026-11-22T10:00:00Z' }] },
    })

    const [item] = await new NotificationHttpRepository().list()

    expect(item).toEqual({ id: 'n-1', kind: 'issue_assigned', message: 'Te asignaron', link: '/issues/1', read: false, createdAt: '2026-11-22T10:00:00Z' })
  })

  it('reads the unread counter and marks notifications as read', async () => {
    vi.spyOn(apiClient, 'get').mockResolvedValue({ data: { data: { count: 3 } } })
    const post = vi.spyOn(apiClient, 'post').mockResolvedValue({ data: {} })
    const repo = new NotificationHttpRepository()

    expect(await repo.unreadCount()).toBe(3)
    await repo.markRead('n-1')
    await repo.markAllRead()

    expect(post).toHaveBeenNthCalledWith(1, '/notifications/n-1/read')
    expect(post).toHaveBeenNthCalledWith(2, '/notifications/read-all')
  })
})
