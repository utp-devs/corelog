import { App } from 'antd'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { savedFilters, listSavedFilters, createSavedFilter, deleteSavedFilter } = vi.hoisted(() => ({
  savedFilters: [{
    id: 'filter-1', name: 'Open bugs', criteria: { status: 'open', issueType: 'bug', query: 'timeout' },
    createdAt: '2026-01-01T00:00:00Z', updatedAt: '2026-01-01T00:00:00Z',
  }],
  listSavedFilters: vi.fn(),
  createSavedFilter: vi.fn(),
  deleteSavedFilter: vi.fn(),
}))

vi.mock('./hooks/useTickets', () => ({ useTickets: () => ({ tickets: [], isLoading: false }) }))
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
    listSavedFilters.mockReset().mockResolvedValue(savedFilters)
    createSavedFilter.mockReset().mockResolvedValue({ ...savedFilters[0], id: 'filter-2', name: 'New filter' })
    deleteSavedFilter.mockReset().mockResolvedValue(undefined)
  })

  it('loads saved criteria into the issue filters and deletes the selected saved filter', async () => {
    renderPage()
    await waitFor(() => expect(listSavedFilters).toHaveBeenCalled())
    const savedSelect = screen.getAllByRole('combobox')[0]
    fireEvent.mouseDown(savedSelect)
    fireEvent.click(await screen.findByText('Open bugs'))

    expect(await screen.findByPlaceholderText('Buscar incidencia por título o descripción')).toHaveValue('timeout')
    fireEvent.click(screen.getByRole('button', { name: 'Eliminar filtro' }))
    await waitFor(() => expect(deleteSavedFilter).toHaveBeenCalledWith('filter-1'))
  })

  it('saves the active issue criteria with a personal name', async () => {
    renderPage()
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
