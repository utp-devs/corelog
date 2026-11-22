import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const repositories = vi.hoisted(() => ({ me: vi.fn() }))

vi.mock('@/adapters/out/http/dashboard-http-repository', () => ({
  DashboardHttpRepository: class {
    me = repositories.me
  },
}))

import { MyDashboardPage } from './MyDashboardPage'

describe('MyDashboardPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    repositories.me.mockResolvedValue({
      assignedOpen: 4,
      assignedByStatus: { open: 2, in_progress: 2 },
      overdue: 1,
      projectCount: 3,
      activeSprints: [{ id: 's-1', name: 'Sprint 1', projectKey: 'CORE', endDate: '2026-11-30' }],
      recentIssues: [{ id: 'i-1', title: 'Revisar exportacion', status: 'open', priority: 'high', projectKey: 'CORE', dueDate: '2026-11-25' }],
    })
  })

  it('shows the personal figures, the active sprints and the urgent issues', async () => {
    render(<MemoryRouter><MyDashboardPage /></MemoryRouter>)

    expect(await screen.findByText('Revisar exportacion')).toBeInTheDocument()
    expect(screen.getByText('Vencidas')).toBeInTheDocument()
    expect(screen.getByText('En curso: 2')).toBeInTheDocument()
    expect(screen.getByText(/CORE · Sprint 1/)).toBeInTheDocument()
  })

  it('shows an error message when the summary cannot be loaded', async () => {
    repositories.me.mockRejectedValue(new Error('network'))
    render(<MemoryRouter><MyDashboardPage /></MemoryRouter>)

    expect(await screen.findByRole('alert')).toHaveTextContent('No se pudo cargar tu resumen')
  })
})
