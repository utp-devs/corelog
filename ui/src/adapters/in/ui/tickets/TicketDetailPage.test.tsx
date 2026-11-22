import { App } from 'antd'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { ticketState, attachmentList, watcherGetState, watcherWatch } = vi.hoisted(() => ({
  ticketState: {
    selected: {
      id: 'issue-1', title: 'Error de prueba', description: 'Descripción de la incidencia', status: 'open',
      priority: 'medium', projectId: 'project-1', issueType: 'bug', parentId: null, dueDate: null,
      storyPoints: null, rank: 'm', closedAt: null, requesterId: 'user-1', assigneeId: null,
      createdAt: '2026-01-01T00:00:00Z', updatedAt: '2026-01-01T00:00:00Z',
    },
    status: 'idle', comments: [], commentsIssueId: 'issue-1', activities: [], activitiesIssueId: 'issue-1', links: [], linksIssueId: 'issue-1',
    fetchTicket: vi.fn(), updateStatus: vi.fn(), assign: vi.fn(), updateDetails: vi.fn(),
    fetchComments: vi.fn().mockResolvedValue(undefined), createComment: vi.fn(), deleteComment: vi.fn(),
    fetchActivities: vi.fn().mockResolvedValue(undefined), fetchLinks: vi.fn().mockResolvedValue(undefined),
    createLink: vi.fn(), deleteLink: vi.fn(),
  },
  attachmentList: vi.fn().mockResolvedValue([]),
  watcherGetState: vi.fn().mockResolvedValue({ isWatching: false, count: 0 }),
  watcherWatch: vi.fn().mockResolvedValue({ isWatching: true, count: 1 }),
}))

vi.mock('@/adapters/in/state/tickets.store', () => ({
  useTicketsStore: (selector: (state: typeof ticketState) => unknown) => selector(ticketState),
}))

vi.mock('@/adapters/out/http/attachment-http-repository', () => ({
  AttachmentHttpRepository: class {
    list = attachmentList
    upload = vi.fn()
    download = vi.fn()
  },
}))

vi.mock('@/adapters/out/http/issue-watcher-http-repository', () => ({
  IssueWatcherHttpRepository: class {
    getState = watcherGetState
    watch = watcherWatch
    unwatch = vi.fn().mockResolvedValue({ isWatching: false, count: 0 })
  },
}))

vi.mock('./hooks/useTeammateSearch', () => ({
  useTeammateSearch: () => ({ results: [], isLoading: false, search: vi.fn() }),
}))

import { TicketDetailPage } from './TicketDetailPage'

describe('TicketDetailPage tabs', () => {
  beforeEach(() => {
    attachmentList.mockClear()
    attachmentList.mockResolvedValue([])
    watcherGetState.mockClear()
    watcherGetState.mockResolvedValue({ isWatching: false, count: 0 })
    watcherWatch.mockClear()
    watcherWatch.mockResolvedValue({ isWatching: true, count: 1 })
  })

  it('shows issue details, comments, activity, and files in separate tabs', async () => {
    render(
      <App>
        <MemoryRouter initialEntries={['/issues/issue-1']}>
          <Routes>
            <Route path="/issues/:id" element={<TicketDetailPage />} />
          </Routes>
        </MemoryRouter>
      </App>,
    )

    expect(await screen.findByText('Metadatos')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('tab', { name: 'Comentarios' }))
    expect(screen.getByPlaceholderText(/Escribe un comentario/)).toBeInTheDocument()

    fireEvent.click(screen.getByRole('tab', { name: 'Actividad' }))
    expect(screen.getByText('Aun no hay actividad')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('tab', { name: 'Archivos' }))
    expect(await screen.findByText('Aún no hay archivos')).toBeInTheDocument()
  })

  it('shows an error state when the attachment list cannot be loaded', async () => {
    attachmentList.mockRejectedValueOnce(new Error('network failure'))
    render(
      <App>
        <MemoryRouter initialEntries={['/issues/issue-1']}>
          <Routes>
            <Route path="/issues/:id" element={<TicketDetailPage />} />
          </Routes>
        </MemoryRouter>
      </App>,
    )

    fireEvent.click(await screen.findByRole('tab', { name: 'Archivos' }))
    expect(await screen.findByText('No se pudieron cargar los archivos')).toBeInTheDocument()
  })

  it('lets a project member follow an issue and updates the follower count', async () => {
    render(
      <App>
        <MemoryRouter initialEntries={['/issues/issue-1']}>
          <Routes>
            <Route path="/issues/:id" element={<TicketDetailPage />} />
          </Routes>
        </MemoryRouter>
      </App>,
    )

    const followButton = await screen.findByRole('button', { name: 'Seguir incidencia' })
    await waitFor(() => expect(followButton).toBeEnabled())
    fireEvent.click(followButton)
    expect(watcherWatch).toHaveBeenCalledWith('issue-1')
    expect(await screen.findByRole('button', { name: 'Dejar de seguir' })).toBeInTheDocument()
    expect(screen.getByText('1 persona sigue esta incidencia')).toBeInTheDocument()
  })
})
