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

import { ProjectBacklogPage } from './ProjectBacklogPage'

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
  beforeEach(() => {
    vi.clearAllMocks()
    repositories.getProject.mockResolvedValue({ id: 'project-1', key: 'CORE', name: 'CoreLog' })
    repositories.list.mockResolvedValue(items)
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
