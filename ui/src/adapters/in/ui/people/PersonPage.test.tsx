import { render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import type { PeopleRepositoryPort } from '@/core/people/application/ports/people-repository.port'
import type { PersonProfile } from '@/core/people/domain/member'
import type { TeamRepositoryPort } from '@/core/teams/application/ports/team-repository.port'
import type { Team } from '@/core/teams/domain/team'
import { AppError } from '@/core/shared/app-error'

const orgState = vi.hoisted(() => ({ org: { id: 'org-1', name: 'Acme' } as { id: string; name: string } | null }))

vi.mock('@/adapters/in/state/org.store', () => ({
  useCurrentOrg: () => ({ org: orgState.org, role: 'admin' }),
  getCurrentOrgId: () => orgState.org?.id ?? null,
}))

import { PersonPage } from './PersonPage'

const person: PersonProfile = {
  userId: 'u-2',
  name: 'Luis Pardo',
  displayName: 'Luis P.',
  email: 'luis@corelog.test',
  jobTitle: 'Analista',
  department: 'Operaciones',
  timezone: 'America/Bogota',
  hasAvatar: false,
  orgRole: 'member',
  joinedAt: '2026-02-01T00:00:00Z',
}

const team: Team = {
  id: 't-1',
  organizationId: 'org-1',
  name: 'Plataforma',
  description: '',
  avatarColor: null,
  createdBy: null,
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z',
  memberCount: 4,
  myRole: 'member',
}

function peopleRepo(overrides: Partial<PeopleRepositoryPort> = {}): PeopleRepositoryPort {
  return {
    listMembers: vi.fn(async () => []),
    getPerson: vi.fn(async () => person),
    ...overrides,
  }
}

function teamRepo(overrides: Partial<Pick<TeamRepositoryPort, 'listForMember'>> = {}) {
  return { listForMember: vi.fn(async () => [team]), ...overrides }
}

function renderPerson(repository: PeopleRepositoryPort, teams = teamRepo(), path = '/people/u-2') {
  render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/people/:id" element={<PersonPage repository={repository} teamRepository={teams} />} />
      </Routes>
    </MemoryRouter>,
  )
  return { teams }
}

describe('PersonPage', () => {
  beforeEach(() => {
    orgState.org = { id: 'org-1', name: 'Acme' }
  })

  it('loads the profile from the people endpoint and the teams of the person', async () => {
    const repo = peopleRepo()
    const { teams } = renderPerson(repo)

    expect(await screen.findByRole('heading', { name: 'Luis P.' })).toBeInTheDocument()
    expect(repo.getPerson).toHaveBeenCalledWith('org-1', 'u-2')
    expect(teams.listForMember).toHaveBeenCalledWith('org-1', 'u-2')
    expect(screen.getByText('Analista')).toBeInTheDocument()
    expect(screen.getByText('Operaciones')).toBeInTheDocument()
    expect(screen.getByText('America/Bogota')).toBeInTheDocument()
    // Wide page with the teams in the side panel (no dead space beside the details).
    expect(document.querySelector('[data-page="person"]')).toHaveAttribute('data-variant', 'wide')
    expect(screen.getByRole('complementary', { name: 'Equipos de la persona' })).toBeInTheDocument()
  })

  it('renders team chips that link to the team page', async () => {
    renderPerson(peopleRepo())
    const link = await screen.findByRole('link', { name: /Plataforma/ })
    expect(link).toHaveAttribute('href', '/teams/t-1')
    expect(screen.getByText('4 miembros')).toBeInTheDocument()
  })

  it('shows the empty teams message when the person has no team', async () => {
    renderPerson(peopleRepo(), teamRepo({ listForMember: vi.fn(async () => []) }))
    expect(await screen.findByText('Sin equipos.')).toBeInTheDocument()
  })

  it('keeps the profile when only the team list fails', async () => {
    renderPerson(peopleRepo(), teamRepo({ listForMember: vi.fn(async () => Promise.reject(new Error('x'))) }))
    expect(await screen.findByRole('heading', { name: 'Luis P.' })).toBeInTheDocument()
    expect(screen.getByText('No se pudieron cargar los equipos.')).toBeInTheDocument()
  })

  it('shows the forbidden page to guests (403 from the people endpoint)', async () => {
    renderPerson(
      peopleRepo({
        getPerson: vi.fn(async () => {
          throw new AppError({ status: 403, code: 'org.forbidden', message: 'x' })
        }),
      }),
    )
    expect(await screen.findByText('No tienes acceso a esta página')).toBeInTheDocument()
  })

  it('shows the not-found page when the person is not a member (404)', async () => {
    renderPerson(
      peopleRepo({
        getPerson: vi.fn(async () => {
          throw new AppError({ status: 404, code: null, message: 'x' })
        }),
      }),
    )
    expect(await screen.findByText('Persona no encontrada')).toBeInTheDocument()
  })

  it('shows an error with retry and loads again on retry', async () => {
    const getPerson = vi
      .fn<PeopleRepositoryPort['getPerson']>()
      .mockRejectedValueOnce(new AppError({ status: 500, code: null, message: 'boom' }))
      .mockResolvedValueOnce(person)
    renderPerson(peopleRepo({ getPerson }))

    const retry = await screen.findByRole('button', { name: /Reintentar/ })
    retry.click()
    expect(await screen.findByRole('heading', { name: 'Luis P.' })).toBeInTheDocument()
    await waitFor(() => expect(getPerson).toHaveBeenCalledTimes(2))
  })

  it('shows the no-organization page when no organization is active', () => {
    orgState.org = null
    renderPerson(peopleRepo())
    expect(screen.getByText('Persona no encontrada')).toBeInTheDocument()
  })
})
