import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const repositories = vi.hoisted(() => ({
  getProject: vi.fn(),
  list: vi.fn(),
  velocity: vi.fn(),
  listIssues: vi.fn(),
  burndown: vi.fn(),
  start: vi.fn(),
  backlog: vi.fn(),
}))

vi.mock('@/adapters/out/http/project-http-repository', () => ({
  ProjectHttpRepository: class {
    get = repositories.getProject
  },
}))

vi.mock('@/adapters/out/http/backlog-http-repository', () => ({
  BacklogHttpRepository: class {
    list = repositories.backlog
  },
}))

vi.mock('@/adapters/out/http/sprint-http-repository', () => ({
  SprintHttpRepository: class {
    list = repositories.list
    velocity = repositories.velocity
    listIssues = repositories.listIssues
    burndown = repositories.burndown
    start = repositories.start
  },
}))

// recharts necesita medidas de DOM que jsdom no calcula; el grafico se prueba aparte.
vi.mock('./BurndownChart', () => ({
  BurndownChart: () => <div data-testid="burndown-chart" />,
}))

import { ProjectSprintsPage } from './ProjectSprintsPage'

const planned = { id: 'sprint-2', projectId: 'project-1', name: 'Sprint 2', goal: '', status: 'planned' }
const active = { id: 'sprint-1', projectId: 'project-1', name: 'Sprint 1', goal: 'Entregar exportacion', status: 'active', startDate: '2026-11-02', endDate: '2026-11-08' }

function renderPage() {
  return render(
    <MemoryRouter initialEntries={['/projects/project-1/sprints']}>
      <Routes><Route path="/projects/:id/sprints" element={<ProjectSprintsPage />} /></Routes>
    </MemoryRouter>,
  )
}

describe('ProjectSprintsPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    repositories.getProject.mockResolvedValue({ id: 'project-1', key: 'CORE', name: 'CoreLog' })
    repositories.list.mockResolvedValue([active, planned])
    repositories.velocity.mockResolvedValue([])
    repositories.listIssues.mockResolvedValue([{ id: 'issue-1', title: 'Exportar CSV', status: 'in_progress', priority: 'high', storyPoints: 3 }])
    repositories.burndown.mockResolvedValue([{ date: '2026-11-02', ideal: 3, remaining: 3 }])
    repositories.backlog.mockResolvedValue([])
  })

  it('shows the active sprint in detail with its issues and burndown', async () => {
    renderPage()

    expect(await screen.findByText('Exportar CSV')).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Sprint 1' })).toBeInTheDocument()
    expect(screen.getByTestId('burndown-chart')).toBeInTheDocument()
  })

  it('starts a planned sprint and reloads the list', async () => {
    repositories.start.mockResolvedValue({ ...planned, status: 'active' })
    renderPage()

    fireEvent.click(await screen.findByRole('button', { name: /Sprint 2/ }))
    fireEvent.click(await screen.findByRole('button', { name: 'Iniciar sprint' }))

    expect(repositories.start).toHaveBeenCalledWith('project-1', 'sprint-2')
    await waitFor(() => expect(repositories.list.mock.calls.length).toBeGreaterThan(1))
  })

  it('reports a conflict when a second sprint cannot start, in Spanish', async () => {
    repositories.start.mockRejectedValue({ response: { status: 409 } })
    renderPage()

    fireEvent.click(await screen.findByRole('button', { name: /Sprint 2/ }))
    fireEvent.click(await screen.findByRole('button', { name: 'Iniciar sprint' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('ya hay otro sprint activo')
  })

  it('does not create a sprint without a name', async () => {
    renderPage()

    fireEvent.click(await screen.findByRole('button', { name: 'Planificar sprint' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('El nombre del sprint es obligatorio')
  })
})
