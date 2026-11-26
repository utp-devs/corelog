import { App } from 'antd'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import type { Ticket } from '@/core/tickets/domain/ticket'

const { savedFilters, listSavedFilters, createSavedFilter, deleteSavedFilter, hook } = vi.hoisted(() => ({
  hook: {
    current: {
      tickets: [] as unknown[],
      total: 0,
      isLoading: false,
      error: undefined as string | undefined,
      refetch: vi.fn(),
    },
  },
  savedFilters: [{
    id: 'filter-1', name: 'Open bugs', criteria: { status: 'open', issueType: 'bug', query: 'timeout' },
    createdAt: '2026-01-01T00:00:00Z', updatedAt: '2026-01-01T00:00:00Z',
  }],
  listSavedFilters: vi.fn(),
  createSavedFilter: vi.fn(),
  deleteSavedFilter: vi.fn(),
}))

vi.mock('./hooks/useTickets', () => ({
  useTickets: () => hook.current,
}))
vi.mock('@/adapters/in/state/tickets.store', () => ({
  useTicketsStore: (selector: (state: { updateRank: () => void }) => unknown) => selector({ updateRank: vi.fn() }),
}))
vi.mock('@/adapters/out/http/saved-filter-http-repository', () => ({
  SavedFilterHttpRepository: class {
    list = listSavedFilters
    create = createSavedFilter
    delete = deleteSavedFilter
  },
}))

import { TicketsListPage } from './TicketsListPage'

const originalMatchMedia = window.matchMedia

const sampleTicket: Ticket = {
  id: 'a1b2c3d4-0000-0000-0000-000000000001',
  title: 'Fallo al guardar el perfil',
  description: 'No guarda los cambios',
  status: 'in_progress',
  priority: 'high',
  projectId: 'project-1',
  issueType: 'bug',
  parentId: null,
  dueDate: null,
  storyPoints: null,
  rank: '00000001',
  closedAt: null,
  requesterId: 'user-1',
  requesterName: 'Ana',
  assigneeId: 'user-2',
  assigneeName: 'Luis Pérez',
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-02T10:00:00Z',
}

function mockViewport(query: string) {
  window.matchMedia = vi.fn((media: string) => ({
    matches: media === query,
    media,
    onchange: null,
    addEventListener: () => {},
    removeEventListener: () => {},
    addListener: () => {},
    removeListener: () => {},
    dispatchEvent: () => false,
  })) as unknown as typeof window.matchMedia
}

function renderPage() {
  render(
    <App>
      <MemoryRouter initialEntries={['/issues']}>
        <Routes><Route path="/issues" element={<TicketsListPage />} /></Routes>
      </MemoryRouter>
    </App>,
  )
}

describe('TicketsListPage saved filters', () => {
  beforeEach(() => {
    window.localStorage.clear()
    hook.current = { tickets: [], total: 0, isLoading: false, error: undefined, refetch: vi.fn() }
    listSavedFilters.mockReset().mockResolvedValue(savedFilters)
    createSavedFilter.mockReset().mockResolvedValue({ ...savedFilters[0], id: 'filter-2', name: 'New filter' })
    deleteSavedFilter.mockReset().mockResolvedValue(undefined)
  })

  it('loads saved criteria into the issue filters and deletes the selected saved filter', async () => {
    renderPage()
    fireEvent.click(await screen.findByRole('button', { name: 'Filtros avanzados' }))
    await waitFor(() => expect(listSavedFilters).toHaveBeenCalled())
    fireEvent.mouseDown(screen.getByText('Cargar filtro guardado'))
    fireEvent.click(await screen.findByText('Open bugs'))

    expect(await screen.findByPlaceholderText('Buscar incidencia por título o descripción')).toHaveValue('timeout')
    fireEvent.click(screen.getByRole('button', { name: 'Eliminar filtro' }))
    await waitFor(() => expect(deleteSavedFilter).toHaveBeenCalledWith('filter-1'))
  })

  it('saves the active issue criteria with a personal name', async () => {
    renderPage()
    fireEvent.click(await screen.findByRole('button', { name: 'Filtros avanzados' }))
    fireEvent.click(screen.getByRole('button', { name: 'Guardar filtro actual' }))
    fireEvent.change(screen.getByPlaceholderText('Nombre del filtro'), { target: { value: 'My current view' } })
    fireEvent.click(screen.getByRole('button', { name: 'Guardar' }))

    await waitFor(() => expect(createSavedFilter).toHaveBeenCalledWith('My current view', {
      status: undefined,
      priority: undefined,
      issueType: undefined,
      query: undefined,
      dueFrom: undefined,
      dueTo: undefined,
    }))
  })
})

describe('TicketsListPage states', () => {
  beforeEach(() => {
    window.localStorage.clear()
    hook.current = { tickets: [], total: 0, isLoading: false, error: undefined, refetch: vi.fn() }
    listSavedFilters.mockReset().mockResolvedValue([])
    window.matchMedia = originalMatchMedia
  })

  afterEach(() => {
    window.matchMedia = originalMatchMedia
  })

  it('shows the error with a retry action that reloads the list', async () => {
    const refetch = vi.fn()
    hook.current = { tickets: [], total: 0, isLoading: false, error: 'Sin conexión con el servidor', refetch }
    renderPage()
    expect(screen.getByRole('alert')).toHaveTextContent('Sin conexión con el servidor')
    fireEvent.click(screen.getByRole('button', { name: 'Reintentar' }))
    expect(refetch).toHaveBeenCalledTimes(1)
  })

  it('shows pagination based on the total across pages', async () => {
    hook.current = { tickets: [sampleTicket], total: 45, isLoading: false, error: undefined, refetch: vi.fn() }
    renderPage()
    await waitFor(() => expect(document.querySelector('.ant-pagination')).not.toBeNull())
    expect(document.querySelector('.ant-pagination-item-3')).not.toBeNull()
  })

  it('hides pagination when there are no results', () => {
    renderPage()
    expect(document.querySelector('.ant-pagination')).toBeNull()
  })

  it('shows an empty state inside the table area when there are no issues', () => {
    renderPage()
    expect(screen.getByText('Aún no hay incidencias')).toBeInTheDocument()
    expect(screen.getAllByRole('button', { name: /Nueva incidencia/ }).length).toBeGreaterThan(0)
  })

  it('offers to clear filters when the active filters match nothing', async () => {
    renderPage()
    fireEvent.change(screen.getByPlaceholderText('Buscar incidencia por título o descripción'), { target: { value: 'zzz' } })
    expect(await screen.findByText('Ninguna incidencia coincide con los filtros')).toBeInTheDocument()
    expect(screen.getAllByRole('button', { name: 'Limpiar filtros' }).length).toBeGreaterThan(0)
  })

  it('shows a loading skeleton while the first page loads', async () => {
    hook.current = { tickets: [], total: 0, isLoading: true, error: undefined, refetch: vi.fn() }
    renderPage()
    expect(await screen.findByRole('status', { name: 'Cargando' }, { timeout: 1000 })).toBeInTheDocument()
  })

  it('renders the table with the key, summary, status and assignee columns on desktop', () => {
    hook.current = { tickets: [sampleTicket], total: 1, isLoading: false, error: undefined, refetch: vi.fn() }
    renderPage()
    expect(screen.getByText('Fallo al guardar el perfil')).toBeInTheDocument()
    expect(screen.getByText('En progreso')).toBeInTheDocument()
    expect(screen.getByText('Luis Pérez')).toBeInTheDocument()
  })

  it('renders cards instead of a table on mobile, with no clipped columns', () => {
    mockViewport('(max-width: 767px)')
    hook.current = { tickets: [sampleTicket], total: 1, isLoading: false, error: undefined, refetch: vi.fn() }
    renderPage()
    expect(screen.queryByRole('table')).not.toBeInTheDocument()
    expect(screen.getByText('Fallo al guardar el perfil')).toBeInTheDocument()
    expect(screen.getByText('En progreso')).toBeInTheDocument()
    expect(screen.getByText('Luis Pérez')).toBeInTheDocument()
  })

  it('renders without console warnings or errors', () => {
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
    const error = vi.spyOn(console, 'error').mockImplementation(() => {})
    hook.current = { tickets: [sampleTicket], total: 1, isLoading: false, error: undefined, refetch: vi.fn() }
    renderPage()
    expect(warn).not.toHaveBeenCalled()
    expect(error).not.toHaveBeenCalled()
    warn.mockRestore()
    error.mockRestore()
  })
})
