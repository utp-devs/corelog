import { render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const repositories = vi.hoisted(() => ({ list: vi.fn() }))

vi.mock('@/adapters/out/http/ticket-http-repository', () => ({
  TicketHttpRepository: class {
    list = repositories.list
  },
}))

import { IssuePicker } from './IssuePicker'

describe('IssuePicker', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    repositories.list.mockResolvedValue({ items: [], total: 0, page: 1, pageSize: 20 })
  })

  it('loads the latest issues on open and never offers the current issue', async () => {
    repositories.list.mockResolvedValue({
      items: [
        { id: 'current', title: 'Actual', projectId: 'p', status: 'open' },
        { id: 'other-1', title: 'Otra incidencia', projectId: 'p', status: 'open' },
      ],
      total: 2,
      page: 1,
      pageSize: 20,
    })
    render(<IssuePicker value="" onChange={vi.fn()} excludeId="current" />)

    await waitFor(() => expect(repositories.list).toHaveBeenCalledWith({ page: 1, pageSize: 20, query: undefined }))
    expect(screen.getByRole('combobox', { name: 'Incidencia a vincular' })).toBeInTheDocument()
  })
})
