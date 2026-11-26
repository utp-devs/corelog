import { render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { AppError } from '@/core/shared/app-error'
import type { PeopleRepositoryPort } from '@/core/people/application/ports/people-repository.port'
import type { OrganizationMember } from '@/core/people/domain/member'

const orgState = vi.hoisted(() => ({ org: { id: 'org-1', name: 'Acme' } as { id: string; name: string } | null }))

vi.mock('@/adapters/in/state/org.store', () => ({
  useCurrentOrg: () => ({ org: orgState.org, role: 'admin' }),
  getCurrentOrgId: () => orgState.org?.id ?? null,
}))

vi.mock('@/adapters/out/http/people-http-repository', () => ({
  PeopleHttpRepository: class {},
}))

const avatarMocks = vi.hoisted(() => ({ getAvatar: vi.fn() }))
vi.mock('@/adapters/out/http/avatar-http-repository', () => ({
  AvatarHttpRepository: class {
    getAvatar = avatarMocks.getAvatar
  },
}))

import { PersonPage } from './PersonPage'

const member: OrganizationMember = {
  userId: 'u-2',
  name: 'Luis Pardo',
  email: 'luis@corelog.test',
  role: 'member',
  joinedAt: '2026-02-01T00:00:00Z',
}

function renderPerson(repository: PeopleRepositoryPort, path = '/people/u-2') {
  render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/people/:id" element={<PersonPage repository={repository} />} />
      </Routes>
    </MemoryRouter>,
  )
}

describe('PersonPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    orgState.org = { id: 'org-1', name: 'Acme' }
    avatarMocks.getAvatar.mockRejectedValue(new AppError({ status: 404, code: 'user.avatar_not_found', message: 'x' }))
  })

  it('shows the public card with the job title and department from the search result', async () => {
    renderPerson({
      listMembers: vi.fn(async () => [member]),
      searchUsers: vi.fn(async () => [
        {
          id: 'u-2',
          name: 'Luis Pardo',
          email: 'luis@corelog.test',
          role: 'user' as const,
          teamId: null,
          displayName: '',
          jobTitle: 'Ingeniero de soporte',
          department: 'Tecnología',
          avatarKey: null,
          timezone: 'America/Bogota',
          createdAt: '2026-02-01T00:00:00Z',
          lastLoginAt: null,
          emailVerifiedAt: null,
        },
      ]),
    })

    expect(await screen.findByRole('heading', { name: 'Luis Pardo' })).toBeInTheDocument()
    expect(screen.getByText('Ingeniero de soporte')).toBeInTheDocument()
    expect(screen.getByText('Tecnología')).toBeInTheDocument()
    expect(screen.getByText('Miembro')).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Personas' })).toHaveAttribute('href', '/people')
  })

  it('still shows the member data when the search for details fails', async () => {
    renderPerson({
      listMembers: vi.fn(async () => [member]),
      searchUsers: vi.fn(async () => {
        throw new Error('offline')
      }),
    })

    expect(await screen.findByRole('heading', { name: 'Luis Pardo' })).toBeInTheDocument()
    expect(screen.getByText('luis@corelog.test')).toBeInTheDocument()
  })

  it('shows a not found page when the person is not in the organization', async () => {
    renderPerson({ listMembers: vi.fn(async () => []), searchUsers: vi.fn(async () => []) })

    expect(await screen.findByText('Persona no encontrada')).toBeInTheDocument()
  })

  it('shows an error with retry when the members cannot be loaded', async () => {
    const listMembers = vi
      .fn<PeopleRepositoryPort['listMembers']>()
      .mockRejectedValueOnce(new Error('offline'))
      .mockResolvedValueOnce([member])
    renderPerson({ listMembers, searchUsers: vi.fn(async () => []) })

    const retry = await screen.findByRole('button', { name: /Reintentar/ })
    retry.click()

    await waitFor(() => expect(screen.getByRole('heading', { name: 'Luis Pardo' })).toBeInTheDocument())
  })
})
