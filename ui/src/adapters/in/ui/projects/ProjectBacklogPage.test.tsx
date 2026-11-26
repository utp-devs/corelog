import { fireEvent, render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const repositories = vi.hoisted(() => ({ getProject: vi.fn(), list: vi.fn(), move: vi.fn() }))

vi.mock('@/adapters/out/http/project-http-repository', () => ({
  ProjectHttpRepository: class {
    get = repositories.getProject
  },
}))

vi.mock('@/adapters/out/http/backlog-http-repository', () => ({
  BacklogHttpRepository: class {
    list = repositories.list
    move = repositories.move
  },
}))

import { AppError } from '@/core/shared/app-error'

import { ProjectBacklogPage } from './ProjectBacklogPage'
import { guardConsole } from './testSupport/consoleGuard'

const items = [
  { id: 'issue-1', projectId: 'project-1', summary: 'Primera', type: 'task', status: 'open', priority: 'high', rank: '000001' },
  { id: 'issue-2', projectId: 'project-1', summary: 'Segunda', type: 'bug', status: 'in_progress', priority: 'low', rank: '000002' },
]

function renderPage() {
  return render(
    <MemoryRouter initialEntries={['/projects/project-1/backlog']}>
      <Routes><Route path="/projects/:id/backlog" element={<ProjectBacklogPage />} /></Routes>
    </MemoryRouter>,
  )
}

describe('ProjectBacklogPage', () => {
  const consoleGuard = guardConsole()

  beforeEach(() => {
    vi.clearAllMocks()
    repositories.getProject.mockResolvedValue({ id: 'project-1', key: 'CORE', name: 'CoreLog' })
    repositories.list.mockResolvedValue(items)
  })

  it('renders the ordered rows and an empty state inside the table when there are no issues', async () => {
    repositories.list.mockResolvedValue([])
    renderPage()

    expect(await screen.findByText('El backlog está vacío')).toBeInTheDocument()
    consoleGuard.expectNoConsoleOutput()
  })

  it('shows the 403 page when the backlog is forbidden', async () => {
    repositories.list.mockRejectedValue(new AppError({ status: 403, message: 'forbidden' }))
    renderPage()

    expect(await screen.findByText('No tienes acceso a esta página')).toBeInTheDocument()
    consoleGuard.expectNoConsoleOutput()
  })

  it('lists the issues in rank order with their position', async () => {
    renderPage()

    expect(await screen.findByText('Primera')).toBeInTheDocument()
    expect(screen.getByText('Segunda')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Subir Primera' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Bajar Segunda' })).toBeDisabled()
  })

  it('moves an issue one position down and shows the reordered list', async () => {
    repositories.move.mockResolvedValue([items[1], items[0]])
    renderPage()

    fireEvent.click(await screen.findByRole('button', { name: 'Bajar Primera' }))

    expect(repositories.move).toHaveBeenCalledWith('project-1', 'issue-1', 1)
    expect(await screen.findByRole('button', { name: 'Subir Primera' })).toBeEnabled()
  })

  it('reports a forbidden reorder in Spanish', async () => {
    repositories.move.mockRejectedValue({ response: { status: 403 } })
    renderPage()

    fireEvent.click(await screen.findByRole('button', { name: 'Bajar Primera' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('No tienes permiso para ordenar el backlog')
  })
})
