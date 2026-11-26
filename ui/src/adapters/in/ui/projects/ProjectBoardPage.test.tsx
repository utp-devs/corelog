import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { AppError } from '@/core/shared/app-error'

const repositories = vi.hoisted(() => ({ getProject: vi.fn(), getBoard: vi.fn(), updateWip: vi.fn(), moveCard: vi.fn() }))

vi.mock('@/adapters/out/http/project-http-repository', () => ({
  ProjectHttpRepository: class {
    get = repositories.getProject
  },
}))

vi.mock('@/adapters/out/http/board-http-repository', () => ({
  BoardHttpRepository: class {
    getProjectBoard = repositories.getBoard
    updateColumnWip = repositories.updateWip
    moveCard = repositories.moveCard
  },
}))

import { ProjectBoardPage } from './ProjectBoardPage'
import { guardConsole } from './testSupport/consoleGuard'
import { installViewport } from './testSupport/viewport'

let restoreViewport: (() => void) | undefined

describe('ProjectBoardPage', () => {
  const consoleGuard = guardConsole()

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

  afterEach(() => {
    restoreViewport?.()
    restoreViewport = undefined
  })

  it('renders ordered columns, counts and issue card details', async () => {
    render(
      <MemoryRouter initialEntries={['/projects/project-1/board']}>
        <Routes><Route path="/projects/:id/board" element={<ProjectBoardPage />} /></Routes>
      </MemoryRouter>,
    )

    expect(await screen.findByText('No carga la pantalla')).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Tablero' })).toBeInTheDocument()
    expect(screen.getByText(/Responsable: Ada Lovelace/)).toBeInTheDocument()
    expect(screen.getByText('3 puntos')).toBeInTheDocument()
    await waitFor(() => expect(repositories.getBoard).toHaveBeenCalledWith('project-1'))
    expect(screen.getByText('Sin incidencias')).toBeInTheDocument()
    consoleGuard.expectNoConsoleOutput()
  })

  it('moves a card to another column with the keyboard select alternative', async () => {
    repositories.moveCard.mockResolvedValue({
      id: 'board-1', projectId: 'project-1', name: 'Default', type: 'kanban',
      columns: [
        { id: 'column-1', name: 'Abierto', status: 'open', position: 0, count: 0, cards: [] },
        { id: 'column-2', name: 'En progreso', status: 'in_progress', position: 1, count: 1, cards: [] },
      ],
    })
    render(
      <MemoryRouter initialEntries={['/projects/project-1/board']}>
        <Routes><Route path="/projects/:id/board" element={<ProjectBoardPage />} /></Routes>
      </MemoryRouter>,
    )

    fireEvent.mouseDown(await screen.findByRole('combobox', { name: 'Mover No carga la pantalla a columna' }))
    fireEvent.click(await screen.findByText('En progreso', { selector: '.ant-select-item-option-content' }))

    await waitFor(() => expect(repositories.moveCard).toHaveBeenCalledWith('project-1', 'issue-1', 'column-2'))
    expect(await screen.findByText('Incidencia movida a En progreso')).toBeInTheDocument()
    consoleGuard.expectNoConsoleOutput()
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

  function renderBoard() {
    return render(
      <MemoryRouter initialEntries={['/projects/project-1/board']}>
        <Routes><Route path="/projects/:id/board" element={<ProjectBoardPage />} /></Routes>
      </MemoryRouter>,
    )
  }

  it('shows skeleton columns with busy feedback while the board loads', async () => {
    repositories.getBoard.mockReturnValue(new Promise(() => undefined))
    renderBoard()

    const loading = await screen.findByLabelText('Cargando tablero')
    expect(loading).toHaveAttribute('aria-busy', 'true')
    expect(screen.queryByRole('region', { name: /Tablero/ })).not.toBeInTheDocument()
    consoleGuard.expectNoConsoleOutput()
  })

  it('shows the 403 page when the board is forbidden', async () => {
    repositories.getBoard.mockRejectedValue(new AppError({ status: 403, message: 'forbidden' }))
    renderBoard()

    expect(await screen.findByText('No tienes acceso a esta página')).toBeInTheDocument()
    expect(screen.queryByRole('region', { name: /Tablero/ })).not.toBeInTheDocument()
    consoleGuard.expectNoConsoleOutput()
  })

  it('shows an empty hint inside a column that has no issues', async () => {
    renderBoard()

    const column = await screen.findByRole('region', { name: 'Tablero Default' })
    expect(within(column).getByText('Arrastra incidencias aquí.')).toBeInTheDocument()
    consoleGuard.expectNoConsoleOutput()
  })

  it('shows an empty state when the project has no columns', async () => {
    repositories.getBoard.mockResolvedValue({ id: 'board-1', projectId: 'project-1', name: 'Default', type: 'kanban', columns: [] })
    renderBoard()

    expect(await screen.findByText('El proyecto todavía no tiene columnas de tablero.')).toBeInTheDocument()
    consoleGuard.expectNoConsoleOutput()
  })

  it('scrolls the columns inside the board region on mobile and lays them out on desktop', async () => {
    restoreViewport = installViewport(390)
    const mobile = renderBoard()
    const mobileBoard = await screen.findByRole('region', { name: 'Tablero Default' })
    expect(mobileBoard).toHaveAttribute('data-scroll', 'inside-board')
    expect(mobileBoard).toHaveAttribute('data-layout', 'mobile-columns')
    expect(mobileBoard).toHaveAttribute('data-scroll-x')
    mobile.unmount()

    restoreViewport()
    restoreViewport = installViewport(1440)
    renderBoard()
    expect(await screen.findByRole('region', { name: 'Tablero Default' })).toHaveAttribute('data-layout', 'desktop-columns')
    consoleGuard.expectNoConsoleOutput()
  })
})
