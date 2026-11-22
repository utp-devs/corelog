import { render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const repositories = vi.hoisted(() => ({ unreadCount: vi.fn(), list: vi.fn(), markRead: vi.fn(), markAllRead: vi.fn() }))

vi.mock('@/adapters/out/http/notification-http-repository', () => ({
  NotificationHttpRepository: class {
    unreadCount = repositories.unreadCount
    list = repositories.list
    markRead = repositories.markRead
    markAllRead = repositories.markAllRead
  },
}))

import { NotificationsBell } from './NotificationsBell'

describe('NotificationsBell', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    repositories.unreadCount.mockResolvedValue(2)
    repositories.list.mockResolvedValue([])
  })

  it('shows the unread count in the bell label', async () => {
    render(<MemoryRouter><NotificationsBell /></MemoryRouter>)

    await waitFor(() => expect(screen.getByRole('button', { name: 'Notificaciones, 2 sin leer' })).toBeInTheDocument())
  })

  it('keeps the plain label when nothing is unread', async () => {
    repositories.unreadCount.mockResolvedValue(0)
    render(<MemoryRouter><NotificationsBell /></MemoryRouter>)

    await waitFor(() => expect(repositories.unreadCount).toHaveBeenCalled())
    expect(screen.getByRole('button', { name: 'Notificaciones' })).toBeInTheDocument()
  })
})
