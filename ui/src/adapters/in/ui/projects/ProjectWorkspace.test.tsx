import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { ProjectHttpRepository } from '@/adapters/out/http/project-http-repository'
import { AppError } from '@/core/shared/app-error'

import { ProjectWorkspace } from './ProjectWorkspace'
import { guardConsole } from './testSupport/consoleGuard'

const project = {
  id: 'p1',
  key: 'DEM',
  name: 'Proyecto demo',
  description: '',
  type: 'scrum',
  visibility: 'private',
} as unknown as Awaited<ReturnType<ProjectHttpRepository['get']>>

function renderWorkspace() {
  return render(
    <MemoryRouter initialEntries={['/projects/p1/board']}>
      <Routes>
        <Route path="/projects/:id" element={<ProjectWorkspace />}>
          <Route path="board" element={<p>Tablero</p>} />
        </Route>
      </Routes>
    </MemoryRouter>,
  )
}

describe('ProjectWorkspace states', () => {
  const consoleGuard = guardConsole()

  beforeEach(() => {
    window.localStorage.clear()
    vi.restoreAllMocks()
  })

  it('shows the project identity, its section tabs and the breadcrumb', async () => {
    vi.spyOn(ProjectHttpRepository.prototype, 'get').mockResolvedValue(project)
    renderWorkspace()

    expect(await screen.findByRole('heading', { name: 'Proyecto demo' })).toBeInTheDocument()
    expect(screen.getByRole('navigation', { name: 'Secciones del proyecto' })).toBeInTheDocument()
    expect(screen.getByRole('navigation', { name: 'Ruta de navegación' })).toHaveTextContent('Proyectos')
    consoleGuard.expectNoConsoleOutput()
  })

  it('shows the 403 page instead of the tabs when the project is forbidden', async () => {
    vi.spyOn(ProjectHttpRepository.prototype, 'get').mockRejectedValue(new AppError({ status: 403, message: 'forbidden' }))
    renderWorkspace()

    expect(await screen.findByText('No tienes acceso a esta página')).toBeInTheDocument()
    expect(screen.queryByRole('navigation', { name: 'Secciones del proyecto' })).not.toBeInTheDocument()
    consoleGuard.expectNoConsoleOutput()
  })

  it('shows an error with retry when the project cannot be loaded for another reason', async () => {
    const get = vi.spyOn(ProjectHttpRepository.prototype, 'get')
      .mockRejectedValueOnce(new AppError({ status: 500, message: 'boom' }))
      .mockResolvedValueOnce(project)
    renderWorkspace()

    expect(await screen.findByRole('alert')).toHaveTextContent('No se pudo abrir el proyecto')
    fireEvent.click(screen.getByRole('button', { name: 'Reintentar' }))

    expect(await screen.findByRole('heading', { name: 'Proyecto demo' })).toBeInTheDocument()
    expect(get).toHaveBeenCalledTimes(2)
  })
})

describe('ProjectWorkspace recents', () => {
  beforeEach(() => {
    window.localStorage.clear()
    vi.spyOn(ProjectHttpRepository.prototype, 'get').mockResolvedValue(project)
  })

  it('records the project in the recent items once its header is loaded', async () => {
    render(
      <MemoryRouter initialEntries={['/projects/p1/board']}>
        <Routes>
          <Route path="/projects/:id" element={<ProjectWorkspace />}>
            <Route path="board" element={<p>Tablero</p>} />
          </Route>
        </Routes>
      </MemoryRouter>,
    )
    expect(await screen.findByText('Proyecto demo')).toBeInTheDocument()
    await waitFor(() => {
      const recent = JSON.parse(window.localStorage.getItem('corelog.recent') ?? '[]') as { kind: string; id: string; subtitle: string }[]
      expect(recent[0]).toMatchObject({ kind: 'project', id: 'p1', subtitle: 'DEM' })
    })
  })
})
