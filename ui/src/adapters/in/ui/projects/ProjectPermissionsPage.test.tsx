import { render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { useAuthStore } from '@/adapters/in/state/auth.store'
import { useProjectsStore } from '@/adapters/in/state/projects.store'
import { ProjectHttpRepository } from '@/adapters/out/http/project-http-repository'
import type { User } from '@/core/auth/domain/user'
import { AppError } from '@/core/shared/app-error'

import { guardConsole } from './testSupport/consoleGuard'
import { ProjectPermissionsPage } from './ProjectPermissionsPage'

function makeUser(): User {
  return {
    id: 'u1',
    name: 'Ana Pérez',
    email: 'ana@example.com',
    role: 'user',
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

function renderPage() {
  return render(
    <MemoryRouter initialEntries={['/projects/p1/permisos']}>
      <Routes>
        <Route path="/projects/:id/permisos" element={<ProjectPermissionsPage />} />
      </Routes>
    </MemoryRouter>,
  )
}

describe('ProjectPermissionsPage', () => {
  const consoleGuard = guardConsole()

  beforeEach(() => {
    vi.restoreAllMocks()
    useAuthStore.setState({ user: makeUser(), status: 'authenticated', sessionChecked: true })
    useProjectsStore.setState({ projects: [], status: 'idle', error: null, members: [], labels: [], components: [] })
  })

  it('shows the role matrix in a table with the current role highlighted', async () => {
    vi.spyOn(ProjectHttpRepository.prototype, 'listMembers').mockResolvedValue([
      { projectId: 'p1', userId: 'u1', role: 'lead', createdAt: '2026-01-01T00:00:00Z' },
      { projectId: 'p1', userId: 'u2', role: 'viewer', createdAt: '2026-01-01T00:00:00Z' },
    ])
    renderPage()

    expect(await screen.findByText('Ver el proyecto')).toBeInTheDocument()
    expect(screen.getAllByText('Tú').length).toBeGreaterThan(0)
    expect(screen.getByRole('heading', { name: 'Matriz de permisos' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Tu rol en este proyecto' })).toBeInTheDocument()
    consoleGuard.expectNoConsoleOutput()
  })

  it('tells users who are not members that they need access', async () => {
    vi.spyOn(ProjectHttpRepository.prototype, 'listMembers').mockResolvedValue([])
    renderPage()

    expect(await screen.findByText('Sin acceso como miembro')).toBeInTheDocument()
    expect(screen.getByText('No apareces como miembro del proyecto. Pide acceso a un líder.')).toBeInTheDocument()
  })

  it('shows the 403 page when the project members are forbidden', async () => {
    vi.spyOn(ProjectHttpRepository.prototype, 'listMembers').mockRejectedValue(new AppError({ status: 403, message: 'forbidden' }))
    renderPage()

    expect(await screen.findByText('No tienes acceso a esta página')).toBeInTheDocument()
    consoleGuard.expectNoConsoleOutput()
  })

  it('shows the skeleton for the role section while the members load', async () => {
    vi.spyOn(ProjectHttpRepository.prototype, 'listMembers').mockReturnValue(new Promise(() => undefined))
    renderPage()

    expect(await screen.findByRole('heading', { name: 'Matriz de permisos' })).toBeInTheDocument()
    expect(screen.queryByText('Sin acceso como miembro')).not.toBeInTheDocument()
    consoleGuard.expectNoConsoleOutput()
  })

  it('keeps the permission matrix and warns when the members fail for another reason', async () => {
    vi.spyOn(ProjectHttpRepository.prototype, 'listMembers').mockRejectedValue(new AppError({ status: 500, message: 'boom' }))
    renderPage()

    expect(await screen.findByText('No se pudieron cargar los miembros. La matriz de permisos sigue vigente.')).toBeInTheDocument()
    expect(screen.getByText('Ver el proyecto')).toBeInTheDocument()
    consoleGuard.expectNoConsoleOutput()
  })
})
