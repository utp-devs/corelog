import { render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const repositories = vi.hoisted(() => ({ getProject: vi.fn(), getBoard: vi.fn(), updateWip: vi.fn() }))

vi.mock('@/adapters/out/http/project-http-repository', () => ({
  ProjectHttpRepository: class {
    get = repositories.getProject
  },
}))

vi.mock('@/adapters/out/http/board-http-repository', () => ({
  BoardHttpRepository: class {
    getProjectBoard = repositories.getBoard
    updateColumnWip = repositories.updateWip
  },
}))

import { ProjectBoardPage } from './ProjectBoardPage'

describe('ProjectBoardPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    repositories.getProject.mockResolvedValue({ id: 'project-1', key: 'CORE', name: 'CoreLog' })
    repositories.getBoard.mockResolvedValue({
      id: 'board-1', projectId: 'project-1', name: 'Default', type: 'kanban',
      columns: [
        { id: 'column-1', name: 'Abierto', status: 'open', position: 0, count: 1, cards: [{
          id: 'issue-1', type: 'bug', summary: 'No carga la pantalla', status: 'open', priority: 'high',
          assignee: 'Ada Lovelace', storyPoints: 3, rank: 'm',
        }] },
        { id: 'column-2', name: 'En progreso', status: 'in_progress', position: 1, count: 0, cards: [] },
      ],
    })
  })

  it('renders ordered columns, counts and issue card details', async () => {
    render(
      <MemoryRouter initialEntries={['/projects/project-1/board']}>
        <Routes><Route path="/projects/:id/board" element={<ProjectBoardPage />} /></Routes>
      </MemoryRouter>,
    )

    expect(await screen.findByText('No carga la pantalla')).toBeInTheDocument()
    expect(screen.getByText('CORE · CoreLog')).toBeInTheDocument()
    expect(screen.getByText(/Responsable: Ada Lovelace/)).toBeInTheDocument()
    expect(screen.getByText('3 puntos')).toBeInTheDocument()
    await waitFor(() => expect(repositories.getBoard).toHaveBeenCalledWith('project-1'))
    expect(screen.getByText('Sin incidencias')).toBeInTheDocument()
  })

  it('shows a useful error when project access or board loading fails', async () => {
    repositories.getBoard.mockRejectedValue(new Error('forbidden'))
    render(
      <MemoryRouter initialEntries={['/projects/project-1/board']}>
        <Routes><Route path="/projects/:id/board" element={<ProjectBoardPage />} /></Routes>
      </MemoryRouter>,
    )

    expect(await screen.findByRole('alert')).toHaveTextContent('No se pudo cargar el tablero')
  })

  it('shows the wip limit next to the count and flags a full column', async () => {
    repositories.getBoard.mockResolvedValue({
      id: 'board-1', projectId: 'project-1', name: 'Default', type: 'kanban',
      columns: [
        { id: 'column-1', name: 'En curso', status: 'in_progress', position: 0, count: 2, wipLimit: 2, cards: [{
          id: 'issue-2', type: 'task', summary: 'Trabajo en curso', status: 'in_progress', priority: 'medium',
          assignee: 'Grace Hopper', rank: 'n',
        }] },
      ],
    })
    render(
      <MemoryRouter initialEntries={['/projects/project-1/board']}>
        <Routes><Route path="/projects/:id/board" element={<ProjectBoardPage />} /></Routes>
      </MemoryRouter>,
    )

    expect(await screen.findByText('2 / 2')).toBeInTheDocument()
    expect(screen.getByText('Trabajo en curso')).toBeInTheDocument()
  })

  it('lets a manager edit the wip limit of a column', async () => {
    repositories.getBoard.mockResolvedValue({
      id: 'board-1', projectId: 'project-1', name: 'Default', type: 'kanban', canManageBoard: true,
      columns: [{ id: 'column-2', name: 'En progreso', status: 'in_progress', position: 1, count: 0, cards: [] }],
    })
    render(
      <MemoryRouter initialEntries={['/projects/project-1/board']}>
        <Routes><Route path="/projects/:id/board" element={<ProjectBoardPage />} /></Routes>
      </MemoryRouter>,
    )

    expect(await screen.findByLabelText('Límite WIP de En progreso')).toBeInTheDocument()
  })

  it('hides the wip editor from users who cannot manage the board', async () => {
    repositories.getBoard.mockResolvedValue({
      id: 'board-1', projectId: 'project-1', name: 'Default', type: 'kanban', canManageBoard: false,
      columns: [{ id: 'column-2', name: 'En progreso', status: 'in_progress', position: 1, count: 0, cards: [] }],
    })
    render(
      <MemoryRouter initialEntries={['/projects/project-1/board']}>
        <Routes><Route path="/projects/:id/board" element={<ProjectBoardPage />} /></Routes>
      </MemoryRouter>,
    )

    expect(await screen.findByText('En progreso')).toBeInTheDocument()
    expect(screen.queryByLabelText('Límite WIP de En progreso')).not.toBeInTheDocument()
  })
})
