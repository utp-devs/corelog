import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { MemoryRouter, Route, Routes, useLocation } from 'react-router-dom'
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
  ProfileHttpRepository: class {},
}))

import { PeoplePage } from './PeoplePage'
import { FORBIDDEN_DIRECTORY_MESSAGE } from './errors'

const members: OrganizationMember[] = [
  { userId: 'u-1', name: 'Ana Torres', displayName: '', email: 'ana@corelog.test', jobTitle: '', department: '', hasAvatar: false, role: 'owner', joinedAt: '2026-01-10T00:00:00Z' },
  { userId: 'u-2', name: 'Luis Pardo', displayName: '', email: 'luis@corelog.test', jobTitle: '', department: '', hasAvatar: false, role: 'member', joinedAt: '2026-02-01T00:00:00Z' },
  { userId: 'u-3', name: 'Marta Ruiz', displayName: '', email: 'marta@corelog.test', jobTitle: '', department: '', hasAvatar: false, role: 'guest', joinedAt: '2026-03-01T00:00:00Z' },
]

function fakeRepository(overrides: Partial<PeopleRepositoryPort> = {}): PeopleRepositoryPort {
  return {
    listMembers: vi.fn(async () => members),
    getPerson: vi.fn(async () => {
      throw new Error('not used')
    }),
    ...overrides,
  }
}

function LocationProbe() {
  const location = useLocation()
  return <p data-testid="location">{location.pathname}</p>
}

function renderPeople(repository: PeopleRepositoryPort) {
  render(
    <MemoryRouter initialEntries={['/people']}>
      <Routes>
        <Route path="/people" element={<PeoplePage repository={repository} />} />
        <Route path="/people/:id" element={<LocationProbe />} />
      </Routes>
    </MemoryRouter>,
  )
}

describe('PeoplePage', () => {
  beforeEach(() => {
    orgState.org = { id: 'org-1', name: 'Acme' }
  })

  it('lists the members with their organization role and email', async () => {
    renderPeople(fakeRepository())

    expect(await screen.findByText('Ana Torres')).toBeInTheDocument()
    expect(screen.getByText('Propietario')).toBeInTheDocument()
    expect(screen.getByText('marta@corelog.test')).toBeInTheDocument()
  })

  it('shows the job title of each member in the directory', async () => {
    renderPeople(
      fakeRepository({
        listMembers: vi.fn(async () => [{ ...members[1], jobTitle: 'Analista de datos' }]),
      }),
    )

    expect(await screen.findByText('Analista de datos')).toBeInTheDocument()
    expect(screen.getByRole('columnheader', { name: 'Cargo' })).toBeInTheDocument()
  })

  it('filters the members by name or email from the search box', async () => {
    renderPeople(fakeRepository())
    await screen.findByText('Ana Torres')

    fireEvent.change(screen.getByRole('textbox', { name: 'Buscar personas' }), { target: { value: 'luis@' } })

    await waitFor(() => expect(screen.queryByText('Ana Torres')).not.toBeInTheDocument())
    expect(screen.getByText('Luis Pardo')).toBeInTheDocument()
  })

  it('shows an empty result message when nothing matches', async () => {
    renderPeople(fakeRepository())
    await screen.findByText('Ana Torres')

    fireEvent.change(screen.getByRole('textbox', { name: 'Buscar personas' }), { target: { value: 'zzz' } })

    expect(await screen.findByText('Sin resultados')).toBeInTheDocument()
  })

  it('shows the empty state when the organization has no members', async () => {
    renderPeople(fakeRepository({ listMembers: vi.fn(async () => []) }))
    expect(await screen.findByText('Aún no hay personas')).toBeInTheDocument()
  })

  it('shows the forbidden message for guests and retries on request', async () => {
    const listMembers = vi
      .fn<PeopleRepositoryPort['listMembers']>()
      .mockRejectedValueOnce(new AppError({ status: 403, code: 'org.forbidden', message: 'x' }))
      .mockResolvedValueOnce(members)
    renderPeople(fakeRepository({ listMembers }))

    const alert = await screen.findByRole('alert')
    expect(within(alert).getByText(FORBIDDEN_DIRECTORY_MESSAGE)).toBeInTheDocument()

    fireEvent.click(within(alert).getByRole('button', { name: /Reintentar/ }))

    expect(await screen.findByText('Ana Torres')).toBeInTheDocument()
    expect(listMembers).toHaveBeenCalledTimes(2)
  })

  it('opens the person card when a row is activated with the keyboard', async () => {
    renderPeople(fakeRepository())
    const row = (await screen.findByText('Luis Pardo')).closest('tr') as HTMLElement

    fireEvent.keyDown(row, { key: 'Enter' })

    expect(await screen.findByTestId('location')).toHaveTextContent('/people/u-2')
  })

  it('asks for an organization when none is active', async () => {
    orgState.org = null
    const repository = fakeRepository()
    renderPeople(repository)

    expect(await screen.findByText('Elige una organización')).toBeInTheDocument()
    expect(repository.listMembers).not.toHaveBeenCalled()
  })
})
