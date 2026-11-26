import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { useAuthStore } from '@/adapters/in/state/auth.store'
import { useProjectsStore } from '@/adapters/in/state/projects.store'
import { useOrgStore } from '@/adapters/in/state/org.store'
import { ProjectHttpRepository } from '@/adapters/out/http/project-http-repository'
import type { User } from '@/core/auth/domain/user'
import type { Project } from '@/core/projects/domain/project'
import { AppError } from '@/core/shared/app-error'

import { guardConsole } from './testSupport/consoleGuard'
import { ProjectsPage } from './ProjectsPage'

function makeUser(role: User['role']): User {
  return {
    id: 'u1',
    name: 'Ana Pérez',
    email: 'ana@example.com',
    role,
    teamId: null,
    displayName: 'Ana',
    jobTitle: '',
    department: '',
    avatarKey: null,
    timezone: 'UTC',
    createdAt: '2026-01-01T00:00:00Z',
    lastLoginAt: null,
    emailVerifiedAt: null,
  }
}

function makeProject(overrides: Partial<Project>): Project {
  return {
    id: 'p-1',
    organizationId: 'o1',
    key: 'CORE',
    name: 'CoreLog',
    description: 'Plataforma de trabajo',
    type: 'kanban',
    leadId: 'u1',
    visibility: 'open',
    archivedAt: null,
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-01T00:00:00Z',
    ...overrides,
  }
}

const projects: Project[] = [
  makeProject({ id: 'p-1', key: 'CORE', name: 'CoreLog', visibility: 'open' }),
  makeProject({ id: 'p-2', key: 'SEC', name: 'Seguridad', visibility: 'private', type: 'scrum' }),
  makeProject({ id: 'p-3', key: 'OLD', name: 'Heredado', visibility: 'open', archivedAt: '2026-02-01T00:00:00Z' }),
]

function renderPage() {
  return render(
    <MemoryRouter>
      <ProjectsPage />
    </MemoryRouter>,
  )
}

describe('ProjectsPage', () => {
  const consoleGuard = guardConsole()

  beforeEach(() => {
    vi.restoreAllMocks()
    useAuthStore.setState({ user: makeUser('admin'), status: 'authenticated', sessionChecked: true })
    useOrgStore.setState({ orgs: [], currentOrgId: 'o1', loaded: true })
    useProjectsStore.setState({ projects: [], status: 'idle', error: null, members: [], labels: [], components: [] })
  })

  it('shows the projects as cards with key, visibility and type, hiding archived ones by default', async () => {
    vi.spyOn(ProjectHttpRepository.prototype, 'list').mockResolvedValue(projects)
    renderPage()

    const grid = await screen.findByRole('list', { name: 'Proyectos' })
    expect(within(grid).getAllByRole('article')).toHaveLength(2)
    const core = screen.getByTestId('project-card-CORE')
    expect(within(core).getByRole('heading', { name: 'CoreLog' })).toBeInTheDocument()
    expect(within(core).getByText('Abierto')).toBeInTheDocument()
    expect(within(screen.getByTestId('project-card-SEC')).getByText('Privado')).toBeInTheDocument()
    expect(screen.queryByText('Heredado')).not.toBeInTheDocument()
    consoleGuard.expectNoConsoleOutput()
  })

  it('filters the cards by visibility', async () => {
    vi.spyOn(ProjectHttpRepository.prototype, 'list').mockResolvedValue(projects)
    renderPage()
    await screen.findByText('CoreLog')

    fireEvent.click(screen.getByText('Privados'))

    expect(screen.queryByText('CoreLog')).not.toBeInTheDocument()
    expect(screen.getByText('Seguridad')).toBeInTheDocument()
  })

  it('shows the archived projects when the status filter asks for them', async () => {
    vi.spyOn(ProjectHttpRepository.prototype, 'list').mockResolvedValue(projects)
    renderPage()
    await screen.findByText('CoreLog')

    fireEvent.mouseDown(screen.getByRole('combobox', { name: 'Filtrar por estado' }))
    fireEvent.click(await screen.findByText('Archivados'))

    expect(await screen.findByText('Heredado')).toBeInTheDocument()
    expect(screen.queryByText('CoreLog')).not.toBeInTheDocument()
  })

  it('shows an empty state with a create action when there are no projects', async () => {
    vi.spyOn(ProjectHttpRepository.prototype, 'list').mockResolvedValue([])
    renderPage()

    expect(await screen.findByText('Aún no hay proyectos')).toBeInTheDocument()
    expect(screen.getAllByRole('button', { name: 'Nuevo proyecto' }).length).toBeGreaterThan(0)
    consoleGuard.expectNoConsoleOutput()
  })

  it('disables project creation for non-admin users with an explanation', async () => {
    useAuthStore.setState({ user: makeUser('user') })
    vi.spyOn(ProjectHttpRepository.prototype, 'list').mockResolvedValue(projects)
    renderPage()
    await screen.findByText('CoreLog')

    expect(screen.getByRole('button', { name: 'Nuevo proyecto' })).toBeDisabled()
  })

  it('shows the 403 page when the API forbids listing projects', async () => {
    vi.spyOn(ProjectHttpRepository.prototype, 'list').mockRejectedValue(new AppError({ status: 403, message: 'forbidden' }))
    renderPage()

    expect(await screen.findByText('No tienes acceso a esta página')).toBeInTheDocument()
    consoleGuard.expectNoConsoleOutput()
  })

  it('shows an error with retry when the list fails for another reason', async () => {
    const list = vi.spyOn(ProjectHttpRepository.prototype, 'list')
      .mockRejectedValueOnce(new AppError({ status: 500, message: 'No se pudieron cargar los proyectos.' }))
      .mockResolvedValueOnce(projects)
    renderPage()

    expect(await screen.findByRole('alert')).toHaveTextContent('No se pudieron cargar los proyectos.')
    fireEvent.click(screen.getByRole('button', { name: 'Reintentar' }))

    expect(await screen.findByText('CoreLog')).toBeInTheDocument()
    expect(list).toHaveBeenCalledTimes(2)
  })

  it('shows the loading skeleton while the projects are requested', async () => {
    vi.spyOn(ProjectHttpRepository.prototype, 'list').mockReturnValue(new Promise(() => undefined))
    renderPage()

    await waitFor(() => expect(screen.getByRole('list', { name: 'Cargando proyectos' })).toHaveAttribute('aria-busy', 'true'))
  })
})
