import { render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { RECENT_STORAGE_KEY } from '@/shared/ui/CommandPalette/recent'
import { guardConsole } from '../projects/testSupport/consoleGuard'

const repositories = vi.hoisted(() => ({ me: vi.fn(), listProjects: vi.fn() }))

vi.mock('@/adapters/out/http/dashboard-http-repository', () => ({
  DashboardHttpRepository: class {
    me = repositories.me
  },
}))

vi.mock('@/adapters/out/http/project-http-repository', () => ({
  ProjectHttpRepository: class {
    list = repositories.listProjects
  },
}))

import { MyDashboardPage } from './MyDashboardPage'

const busySummary = {
  assignedOpen: 4,
  assignedByStatus: { open: 2, in_progress: 2 },
  overdue: 1,
  projectCount: 3,
  activeSprints: [{ id: 's-1', name: 'Sprint 1', projectKey: 'CORE', endDate: '2026-11-30' }],
  recentIssues: [{ id: 'i-1', title: 'Revisar exportacion', status: 'open', priority: 'high', projectKey: 'CORE', dueDate: '2026-11-25' }],
}

const idleSummary = {
  assignedOpen: 0,
  assignedByStatus: {},
  overdue: 0,
  projectCount: 1,
  activeSprints: [],
  recentIssues: [],
}

const project = { id: 'p-1', key: 'CORE', name: 'CoreLog', description: '', type: 'kanban', visibility: 'open', archivedAt: null }

describe('MyDashboardPage', () => {
  const consoleGuard = guardConsole()

  beforeEach(() => {
    vi.clearAllMocks()
    window.localStorage.clear()
    repositories.me.mockResolvedValue(busySummary)
    repositories.listProjects.mockResolvedValue([project])
  })

  it('shows the personal figures, the active sprints and the urgent issues', async () => {
    render(<MemoryRouter><MyDashboardPage /></MemoryRouter>)

    expect(await screen.findByText('Revisar exportacion')).toBeInTheDocument()
    expect(screen.getByText('Vencidas')).toBeInTheDocument()
    expect(screen.getByText('En curso: 2')).toBeInTheDocument()
    expect(screen.getByText(/CORE · Sprint 1/)).toBeInTheDocument()
    consoleGuard.expectNoConsoleOutput()
  })

  it('shows the status breakdown as an accessible data table too', async () => {
    render(<MemoryRouter><MyDashboardPage /></MemoryRouter>)

    await screen.findByText('Revisar exportacion')
    expect(screen.getByRole('table', { name: 'Datos: Por estado' })).toHaveTextContent('En curso')
  })

  it('shows the getting-started card instead of an empty page when there is no assigned work', async () => {
    repositories.me.mockResolvedValue(idleSummary)
    render(<MemoryRouter><MyDashboardPage /></MemoryRouter>)

    expect(await screen.findByRole('heading', { name: 'Primeros pasos' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Crea tu primer proyecto' })).toHaveAttribute('href', '/projects')
    expect(screen.getByText('Sin trabajo asignado.')).toBeInTheDocument()
    consoleGuard.expectNoConsoleOutput()
  })

  it('does not show the getting-started card to users with assigned work', async () => {
    render(<MemoryRouter><MyDashboardPage /></MemoryRouter>)

    await screen.findByText('Revisar exportacion')
    expect(screen.queryByRole('heading', { name: 'Primeros pasos' })).not.toBeInTheDocument()
  })

  it('lists the projects with a link to their board', async () => {
    repositories.me.mockResolvedValue(idleSummary)
    render(<MemoryRouter><MyDashboardPage /></MemoryRouter>)

    expect(await screen.findByRole('link', { name: 'CoreLog' })).toHaveAttribute('href', '/projects/p-1/board')
  })

  it('shows the recently visited items from the local history', async () => {
    window.localStorage.setItem(RECENT_STORAGE_KEY, JSON.stringify([
      { kind: 'issue', id: 'i-9', title: 'Incidencia vista', subtitle: 'CORE-9', link: '/issues/i-9' },
    ]))
    render(<MemoryRouter><MyDashboardPage /></MemoryRouter>)

    expect(await screen.findByRole('link', { name: 'Incidencia vista' })).toHaveAttribute('href', '/issues/i-9')
    await waitFor(() => expect(screen.getByText('Incidencia')).toBeInTheDocument())
  })

  it('shows an error message when the summary cannot be loaded', async () => {
    repositories.me.mockRejectedValue(new Error('network'))
    render(<MemoryRouter><MyDashboardPage /></MemoryRouter>)

    expect(await screen.findByRole('alert')).toHaveTextContent('No se pudo cargar tu resumen')
  })
})
