import { App } from 'antd'
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

const { ticketState, attachmentList, watcherGetState, watcherWatch } = vi.hoisted(() => ({
  ticketState: {
    selected: {
      id: 'issue-1', title: 'Error de prueba', description: 'Descripción de la incidencia', status: 'open',
      priority: 'medium', projectId: 'project-1', issueType: 'bug', parentId: null, dueDate: null,
      storyPoints: null, rank: 'm', closedAt: null, requesterId: 'user-1', requesterName: 'Ana',
      assigneeId: null, assigneeName: null,
      createdAt: '2026-01-01T00:00:00Z', updatedAt: '2026-01-01T00:00:00Z',
    },
    status: 'idle' as 'idle' | 'loading' | 'error',
    error: null as string | null,
    comments: [], commentsIssueId: 'issue-1', activities: [], activitiesIssueId: 'issue-1', links: [], linksIssueId: 'issue-1',
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

const originalMatchMedia = window.matchMedia

function mockStacked(stacked: boolean) {
  window.matchMedia = vi.fn((query: string) => ({
    matches: stacked && query === '(max-width: 991.98px)',
    media: query,
    onchange: null,
    addEventListener: () => {},
    removeEventListener: () => {},
    addListener: () => {},
    removeListener: () => {},
    dispatchEvent: () => false,
  })) as unknown as typeof window.matchMedia
}

function renderDetail() {
  return render(
    <App>
      <MemoryRouter initialEntries={['/issues/issue-1']}>
        <Routes>
          <Route path="/issues/:id" element={<TicketDetailPage />} />
        </Routes>
      </MemoryRouter>
    </App>,
  )
}

describe('TicketDetailPage', () => {
  beforeEach(() => {
    window.localStorage.clear()
    ticketState.status = 'idle'
    ticketState.error = null
    attachmentList.mockClear()
    attachmentList.mockResolvedValue([])
    watcherGetState.mockClear()
    watcherGetState.mockResolvedValue({ isWatching: false, count: 0 })
    watcherWatch.mockClear()
    watcherWatch.mockResolvedValue({ isWatching: true, count: 1 })
    mockStacked(false)
  })

  afterEach(() => {
    window.matchMedia = originalMatchMedia
  })

  it('shows description, details, links, files, comments and history as sections', async () => {
    renderDetail()

    expect(await screen.findByRole('heading', { name: 'Detalles' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Vínculos' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Adjuntos' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Comentarios' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Historial' })).toBeInTheDocument()
    expect(await screen.findByText('Aún no hay archivos')).toBeInTheDocument()
    expect(screen.getByText('Aún no hay comentarios')).toBeInTheDocument()
    expect(screen.getByText('Aún no hay actividad')).toBeInTheDocument()
    expect(screen.getByText('Aún no hay vínculos')).toBeInTheDocument()
  })

  it('lists main content before links and the side panel after the main column on mobile', async () => {
    mockStacked(true)
    renderDetail()
    await screen.findByRole('heading', { name: 'Detalles' })

    const headings = screen.getAllByRole('heading', { level: 2 }).map((heading) => heading.textContent)
    const position = (name: string) => headings.indexOf(name)
    expect(position('Descripción')).toBeGreaterThanOrEqual(0)
    expect(position('Descripción')).toBeLessThan(position('Detalles'))
    expect(position('Detalles')).toBeLessThan(position('Vínculos'))
    expect(position('Vínculos')).toBeLessThan(position('Historial'))
    expect(position('Historial')).toBeLessThan(position('Estado y prioridad'))

    const page = document.querySelector('[data-page="issue-detail"]')
    expect(page).toHaveAttribute('data-stacked', 'true')
    expect(page).toHaveAttribute('data-has-sidebar', 'true')
  })

  it('keeps the side panel beside the main column on desktop', async () => {
    renderDetail()
    await screen.findByRole('heading', { name: 'Detalles' })
    expect(document.querySelector('[data-page="issue-detail"]')).toHaveAttribute('data-stacked', 'false')
  })

  it('shows a loading skeleton while the issue is loading', () => {
    ticketState.status = 'loading'
    renderDetail()
    expect(screen.getByRole('status', { name: 'Cargando' })).toBeInTheDocument()
  })

  it('shows an error with retry when the issue cannot be loaded', () => {
    ticketState.status = 'error'
    ticketState.error = 'No se pudo cargar la incidencia'
    renderDetail()
    expect(screen.getByRole('alert')).toHaveTextContent('No se pudo cargar la incidencia')
    fireEvent.click(screen.getByRole('button', { name: 'Reintentar' }))
    expect(ticketState.fetchTicket).toHaveBeenCalledWith('issue-1')
  })

  it('renders without console warnings or errors', async () => {
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
    const error = vi.spyOn(console, 'error').mockImplementation(() => {})
    renderDetail()
    await screen.findByRole('heading', { name: 'Detalles' })
    await screen.findByText('Aún no hay actividad')
    expect(warn).not.toHaveBeenCalled()
    expect(error).not.toHaveBeenCalled()
    warn.mockRestore()
    error.mockRestore()
  })

  it('records the issue in the recent items once it is shown', async () => {
    renderDetail()
    expect(await screen.findByRole('heading', { name: 'Detalles' })).toBeInTheDocument()
    const recent = JSON.parse(window.localStorage.getItem('corelog.recent') ?? '[]') as { kind: string; id: string; link: string }[]
    expect(recent[0]).toMatchObject({ kind: 'issue', id: 'issue-1', link: '/issues/issue-1' })
  })

  it('shows an error state when the attachment list cannot be loaded', async () => {
    attachmentList.mockRejectedValueOnce(new Error('network failure'))
    renderDetail()
    expect(await screen.findByText('No se pudieron cargar los archivos')).toBeInTheDocument()
  })

  it('lets a project member follow an issue and updates the follower count', async () => {
    renderDetail()

    await waitFor(() => expect(screen.getByRole('button', { name: 'Seguir incidencia' })).not.toHaveClass('ant-btn-loading'))
    fireEvent.click(screen.getByRole('button', { name: 'Seguir incidencia' }))
    expect(watcherWatch).toHaveBeenCalledWith('issue-1')
    expect(await screen.findByRole('button', { name: 'Dejar de seguir' })).toBeInTheDocument()
    expect(screen.getByText('1 persona sigue esta incidencia')).toBeInTheDocument()
  })

  it('shows the description in view mode and opens the editor from the section action', async () => {
    renderDetail()
    const section = (await screen.findByRole('heading', { name: 'Descripción' })).closest('section')
    expect(section).not.toBeNull()
    expect(within(section as HTMLElement).getByText('Descripción de la incidencia')).toBeInTheDocument()
    fireEvent.click(within(section as HTMLElement).getByRole('button', { name: /Editar/ }))
    expect(within(section as HTMLElement).getByRole('button', { name: 'Guardar' })).toBeInTheDocument()
  })
})
