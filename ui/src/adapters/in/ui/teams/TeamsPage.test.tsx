import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { AppError } from '@/core/shared/app-error'
import type { OrgRole, OrgWhoCan } from '@/core/organizations/domain/organization'
import type { Team } from '@/core/teams/domain/team'

const store = vi.hoisted(() => ({
  teams: [] as Team[],
  status: 'idle' as 'idle' | 'loading' | 'error',
  error: null as AppError | null,
  fetchTeams: vi.fn(),
  createTeam: vi.fn(),
}))

const org = vi.hoisted(() => ({
  role: 'admin' as OrgRole | null,
  policy: 'admins' as OrgWhoCan,
}))

const toastSpy = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn(), info: vi.fn() }))

vi.mock('@/adapters/in/state/teams.store', () => ({
  useTeamsStore: (selector: (state: unknown) => unknown) => selector(store),
}))

vi.mock('@/adapters/in/state/org.store', () => ({
  useCurrentOrg: () => ({
    org: { id: 'org-1', settings: { whoCanCreateTeams: org.policy } },
    role: org.role,
  }),
}))

vi.mock('@/shared/ui/Toaster/toast', () => ({ toast: toastSpy }))

import { TeamsPage } from './TeamsPage'

const team = (id: string, name: string, description = ''): Team => ({
  id,
  organizationId: 'org-1',
  name,
  description,
  avatarColor: null,
  createdBy: null,
  createdAt: '2026-11-01T10:00:00Z',
  updatedAt: '2026-11-01T10:00:00Z',
})

function renderPage() {
  return render(
    <MemoryRouter>
      <TeamsPage />
    </MemoryRouter>,
  )
}

function queryCreateButton() {
  return screen.queryByRole('button', { name: /^Crear equipo$/ })
}

describe('TeamsPage', () => {
  beforeEach(() => {
    store.teams = []
    store.status = 'idle'
    store.error = null
    store.fetchTeams.mockReset().mockResolvedValue(undefined)
    store.createTeam.mockReset().mockResolvedValue(team('new', 'Nuevo'))
    org.role = 'admin'
    org.policy = 'admins'
    toastSpy.success.mockReset()
    toastSpy.error.mockReset()
  })

  describe('create permission matrix (organization role x policy)', () => {
    const cases: [OrgRole | null, OrgWhoCan, boolean][] = [
      ['owner', 'members', true],
      ['owner', 'admins', true],
      ['admin', 'members', true],
      ['admin', 'admins', true],
      ['member', 'members', true],
      ['member', 'admins', false],
      ['guest', 'members', false],
      ['guest', 'admins', false],
      [null, 'members', false],
    ]

    it.each(cases)('role %s with policy %s shows the create button: %s', (role, policy, allowed) => {
      org.role = role
      org.policy = policy
      store.teams = [team('t1', 'Alfa')]

      renderPage()

      expect(queryCreateButton() !== null).toBe(allowed)
    })
  })

  it('shows a loading skeleton with the final card geometry while loading', () => {
    store.status = 'loading'

    renderPage()

    expect(screen.getByRole('status', { name: 'Cargando equipos' })).toHaveAttribute('aria-busy', 'true')
  })

  it('shows an error with retry that refetches the teams', () => {
    store.status = 'error'
    store.error = new AppError({ status: 500, message: 'Fallo del servidor' })

    renderPage()

    expect(screen.getByRole('alert')).toHaveTextContent('Fallo del servidor')
    fireEvent.click(screen.getByRole('button', { name: 'Reintentar' }))
    expect(store.fetchTeams).toHaveBeenCalledTimes(2)
  })

  it('offers the creation CTA in the empty state for allowed roles', () => {
    renderPage()

    expect(screen.getByText('Aún no hay equipos')).toBeInTheDocument()
    expect(screen.getAllByRole('button', { name: /^Crear equipo$/ })).toHaveLength(1)
  })

  it('explains the empty state without a CTA for guests', () => {
    org.role = 'guest'

    renderPage()

    expect(screen.getByText('Pide a un administrador de la organización que cree un equipo.')).toBeInTheDocument()
    expect(queryCreateButton()).toBeNull()
  })

  it('renders one card per team linking to the team page', () => {
    store.teams = [team('t1', 'Plataforma', 'Infraestructura'), team('t2', 'Soporte')]

    renderPage()

    const cards = screen.getAllByTestId('team-card')
    expect(cards).toHaveLength(2)
    expect(cards[0]).toHaveAttribute('href', '/teams/t1')
    expect(within(cards[0]).getByText('Infraestructura')).toBeInTheDocument()
    expect(within(cards[1]).getByText('Sin descripción')).toBeInTheDocument()
  })

  it('filters the cards with the search box', async () => {
    store.teams = [team('t1', 'Plataforma'), team('t2', 'Soporte')]

    renderPage()
    fireEvent.change(screen.getByRole('textbox', { name: 'Buscar equipos' }), { target: { value: 'soporte' } })

    await waitFor(() =>
      expect(screen.getAllByTestId('team-card').map((card) => card.getAttribute('href'))).toEqual(['/teams/t2']),
    )
  })

  it('shows a no-match state when the search finds nothing', async () => {
    store.teams = [team('t1', 'Plataforma')]

    renderPage()
    fireEvent.change(screen.getByRole('textbox', { name: 'Buscar equipos' }), { target: { value: 'zzz' } })

    expect(await screen.findByText('Ningún equipo coincide con la búsqueda')).toBeInTheDocument()
  })

  describe('create flow', () => {
    function openCreateModal() {
      store.teams = [team('t1', 'Plataforma')]
      renderPage()
      fireEvent.click(queryCreateButton() as HTMLElement)
      return screen.findByRole('dialog')
    }

    it('validates the name length before calling the server', async () => {
      const dialog = await openCreateModal()
      fireEvent.change(within(dialog).getByLabelText('Nombre del equipo'), { target: { value: 'A' } })
      fireEvent.click(within(dialog).getByRole('button', { name: 'Guardar' }))

      expect(await within(dialog).findByText('El nombre debe tener entre 2 y 120 caracteres.')).toBeInTheDocument()
      expect(store.createTeam).not.toHaveBeenCalled()
    })

    it('creates the team and closes the modal on success', async () => {
      const dialog = await openCreateModal()
      fireEvent.change(within(dialog).getByLabelText('Nombre del equipo'), { target: { value: 'Operaciones' } })
      fireEvent.change(within(dialog).getByLabelText('Descripción'), { target: { value: 'Turnos y guardias' } })
      fireEvent.click(within(dialog).getByRole('button', { name: 'Guardar' }))

      await waitFor(() =>
        expect(store.createTeam).toHaveBeenCalledWith({
          name: 'Operaciones',
          description: 'Turnos y guardias',
          avatarColor: null,
        }),
      )
      await waitFor(() => expect(toastSpy.success).toHaveBeenCalledWith('Equipo creado'))
    })

    it('maps team.name_taken to the name field', async () => {
      store.createTeam.mockRejectedValueOnce(new AppError({ status: 409, code: 'team.name_taken', message: 'x' }))
      const dialog = await openCreateModal()
      fireEvent.change(within(dialog).getByLabelText('Nombre del equipo'), { target: { value: 'Plataforma' } })
      fireEvent.click(within(dialog).getByRole('button', { name: 'Guardar' }))

      expect(
        await within(dialog).findByText('Ya existe un equipo con ese nombre en la organización.'),
      ).toBeInTheDocument()
      expect(toastSpy.success).not.toHaveBeenCalled()
    })

    it('shows team.creation_not_allowed as an alert inside the modal', async () => {
      store.createTeam.mockRejectedValueOnce(
        new AppError({ status: 403, code: 'team.creation_not_allowed', message: 'No puedes crear equipos.', serverMessage: 'No puedes crear equipos.' }),
      )
      const dialog = await openCreateModal()
      fireEvent.change(within(dialog).getByLabelText('Nombre del equipo'), { target: { value: 'Ventas' } })
      fireEvent.click(within(dialog).getByRole('button', { name: 'Guardar' }))

      expect(await within(dialog).findByRole('alert')).toHaveTextContent('No puedes crear equipos.')
    })
  })
})
