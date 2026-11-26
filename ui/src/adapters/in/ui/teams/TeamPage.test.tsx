import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { AppError } from '@/core/shared/app-error'
import type { OrgRole } from '@/core/organizations/domain/organization'
import type { Team, TeamMemberRole } from '@/core/teams/domain/team'

interface MemberFixture {
  teamId: string
  userId: string
  role: TeamMemberRole
  createdAt: string
  name: string
  email: string
}

const store = vi.hoisted(() => ({
  currentTeam: null as Team | null,
  currentTeamStatus: 'idle' as 'idle' | 'loading' | 'error',
  members: [] as MemberFixture[],
  membersStatus: 'idle' as 'idle' | 'loading' | 'error',
  membersError: null as AppError | null,
  error: null as AppError | null,
  fetchTeam: vi.fn(),
  fetchMembers: vi.fn(),
  updateTeam: vi.fn(),
  removeTeam: vi.fn(),
  addMember: vi.fn(),
  updateMemberRole: vi.fn(),
  removeMember: vi.fn(),
}))

const session = vi.hoisted(() => ({
  viewerId: 'me' as string | null,
  orgRole: 'member' as OrgRole | null,
}))

const toastSpy = vi.hoisted(() => ({ success: vi.fn(), error: vi.fn(), info: vi.fn() }))

vi.mock('@/adapters/in/state/teams.store', () => ({
  useTeamsStore: (selector: (state: unknown) => unknown) => selector(store),
}))

vi.mock('@/adapters/in/state/auth.store', () => ({
  useAuthStore: (selector: (state: unknown) => unknown) => selector({ user: { id: session.viewerId } }),
}))

vi.mock('@/adapters/in/state/org.store', () => ({
  useCurrentOrg: () => ({ org: { id: 'org-1' }, role: session.orgRole }),
}))

vi.mock('@/adapters/out/http/user-directory-http-repository', () => ({
  UserDirectoryHttpRepository: class {
    searchTeammates = vi.fn().mockResolvedValue([])
  },
}))

vi.mock('@/shared/ui/Toaster/toast', () => ({ toast: toastSpy }))

import { TeamPage } from './TeamPage'

const TEAM: Team = {
  id: 'team-1',
  organizationId: 'org-1',
  name: 'Soporte Nivel 1',
  description: 'Atención de primera línea',
  avatarColor: null,
  createdBy: 'me',
  createdAt: '2026-11-01T10:00:00Z',
  updatedAt: '2026-11-01T10:00:00Z',
}

const MEMBERS: MemberFixture[] = [
  { teamId: 'team-1', userId: 'me', role: 'member', createdAt: '', name: 'Ana Pérez', email: 'ana@example.com' },
  { teamId: 'team-1', userId: 'u2', role: 'maintainer', createdAt: '', name: 'Luis Soto', email: 'luis@example.com' },
]

function setMatchMedia(mobile: boolean) {
  window.matchMedia = vi.fn().mockImplementation((query: string) => ({
    matches: mobile && query.includes('767'),
    media: query,
    onchange: null,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    addListener: vi.fn(),
    removeListener: vi.fn(),
    dispatchEvent: vi.fn(),
  })) as unknown as typeof window.matchMedia
}

function renderPage(path = '/teams/team-1') {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/teams/:id" element={<TeamPage />} />
        <Route path="/teams" element={<p>Listado de equipos</p>} />
      </Routes>
    </MemoryRouter>,
  )
}

describe('TeamPage', () => {
  beforeEach(() => {
    setMatchMedia(false)
    store.currentTeam = TEAM
    store.currentTeamStatus = 'idle'
    store.members = MEMBERS.map((m) => ({ ...m }))
    store.membersStatus = 'idle'
    store.membersError = null
    store.error = null
    store.fetchTeam.mockReset().mockResolvedValue(undefined)
    store.fetchMembers.mockReset().mockResolvedValue(undefined)
    store.updateTeam.mockReset().mockResolvedValue(TEAM)
    store.removeTeam.mockReset().mockResolvedValue(undefined)
    store.removeMember.mockReset().mockResolvedValue(undefined)
    store.updateMemberRole.mockReset().mockResolvedValue(undefined)
    session.viewerId = 'me'
    session.orgRole = 'member'
    toastSpy.success.mockReset()
    toastSpy.error.mockReset()
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('loads the team and its members for the route id', () => {
    renderPage()

    expect(store.fetchTeam).toHaveBeenCalledWith('team-1')
    expect(store.fetchMembers).toHaveBeenCalledWith('team-1')
    expect(screen.getByRole('heading', { level: 1, name: 'Soporte Nivel 1' })).toBeInTheDocument()
  })

  it('only offers the members tab; projects and calendar tabs are not shown', () => {
    renderPage()

    expect(screen.getByRole('tab', { name: 'Miembros' })).toBeInTheDocument()
    expect(screen.queryByRole('tab', { name: 'Proyectos' })).toBeNull()
    expect(screen.queryByRole('tab', { name: 'Calendario' })).toBeNull()
  })

  it('shows the details panel with the creator and creation date', () => {
    renderPage()

    const details = screen.getByRole('complementary', { name: 'Panel lateral' })
    expect(within(details).getByText('Detalles')).toBeInTheDocument()
    expect(within(details).getByText('Ana Pérez')).toBeInTheDocument()
    expect(within(details).getByText('Fecha de creación')).toBeInTheDocument()
  })

  describe('permissions by role', () => {
    it('lets a team maintainer edit, delete and manage members', () => {
      session.viewerId = 'u2'
      renderPage()

      // Luis is the maintainer of the team in MEMBERS.
      expect(screen.getByRole('button', { name: 'Editar equipo' })).toBeInTheDocument()
      expect(screen.getByRole('button', { name: 'Eliminar equipo' })).toBeInTheDocument()
      expect(screen.getByRole('group', { name: 'Agregar miembro' })).toBeInTheDocument()
      expect(screen.getByRole('combobox', { name: 'Rol de Ana Pérez' })).toBeInTheDocument()
      expect(screen.getByRole('button', { name: 'Quitar a Ana Pérez del equipo' })).toBeInTheDocument()
    })

    it('keeps a plain team member read-only', () => {
      renderPage()

      expect(screen.queryByRole('button', { name: 'Editar equipo' })).toBeNull()
      expect(screen.queryByRole('button', { name: 'Eliminar equipo' })).toBeNull()
      expect(screen.queryByRole('group', { name: 'Agregar miembro' })).toBeNull()
      expect(screen.queryByRole('combobox', { name: 'Rol de Luis Soto' })).toBeNull()
      expect(screen.queryByRole('button', { name: /Quitar a/ })).toBeNull()
      expect(screen.getByText('Responsable')).toBeInTheDocument()
      expect(screen.getByText('Solo lectura: los responsables del equipo gestionan a sus miembros.')).toBeInTheDocument()
    })

    it('lets an organization admin manage a team they do not belong to', () => {
      session.viewerId = 'someone-else'
      session.orgRole = 'admin'
      renderPage()

      expect(screen.getByRole('button', { name: 'Editar equipo' })).toBeInTheDocument()
      expect(screen.getByRole('group', { name: 'Agregar miembro' })).toBeInTheDocument()
    })

    it('keeps guests read-only', () => {
      session.viewerId = 'guest-user'
      session.orgRole = 'guest'
      renderPage()

      expect(screen.queryByRole('button', { name: 'Editar equipo' })).toBeNull()
      expect(screen.queryByRole('group', { name: 'Agregar miembro' })).toBeNull()
    })
  })

  it('changes a member role through the role select', async () => {
    session.viewerId = 'u2'
    renderPage()

    fireEvent.mouseDown(screen.getByRole('combobox', { name: 'Rol de Ana Pérez' }))
    fireEvent.click(await screen.findByText('Responsable', { selector: '.ant-select-item-option-content' }))

    await waitFor(() => expect(store.updateMemberRole).toHaveBeenCalledWith('team-1', 'me', 'maintainer'))
  })

  it('removes a member from the team', async () => {
    session.viewerId = 'u2'
    renderPage()

    fireEvent.click(screen.getByRole('button', { name: 'Quitar a Ana Pérez del equipo' }))

    await waitFor(() => expect(store.removeMember).toHaveBeenCalledWith('team-1', 'me'))
    await waitFor(() => expect(toastSpy.success).toHaveBeenCalled())
  })

  it('explains a forbidden members read instead of showing an empty table', () => {
    store.membersStatus = 'error'
    store.membersError = new AppError({ status: 403, code: 'team.forbidden', message: 'no' })
    renderPage()

    expect(screen.getByRole('alert')).toHaveTextContent('No tienes permiso para ver los miembros de este equipo.')
  })

  it('shows an empty members state', () => {
    store.members = []
    renderPage()

    expect(screen.getByText('Este equipo aún no tiene miembros')).toBeInTheDocument()
  })

  it('shows a not-found message when the team cannot be read', () => {
    store.currentTeam = null
    store.currentTeamStatus = 'error'
    store.error = new AppError({ status: 404, code: 'team.not_found', message: 'no' })
    renderPage()

    expect(screen.getByRole('alert')).toHaveTextContent('Este equipo no existe o no tienes acceso.')
  })

  it('renders member cards instead of a table on mobile', () => {
    setMatchMedia(true)
    session.viewerId = 'u2'
    renderPage()

    expect(screen.queryByRole('table')).toBeNull()
    expect(screen.getAllByText('Ana Pérez').length).toBeGreaterThan(0)
    expect(screen.getByRole('button', { name: 'Quitar a Ana Pérez del equipo' })).toBeInTheDocument()
  })

  describe('delete confirmation', () => {
    it('does not delete until the team name is typed, then removes and returns to the list', async () => {
      session.viewerId = 'u2'
      renderPage()

      fireEvent.click(screen.getByRole('button', { name: 'Eliminar equipo' }))
      const dialog = await screen.findByRole('dialog')
      const confirm = within(dialog).getByRole('button', { name: 'Eliminar equipo' })
      expect(confirm).toBeDisabled()

      fireEvent.change(within(dialog).getByRole('textbox'), { target: { value: 'Soporte Nivel 1' } })
      fireEvent.click(confirm)

      await waitFor(() => expect(store.removeTeam).toHaveBeenCalledWith('team-1'))
      expect(await screen.findByText('Listado de equipos')).toBeInTheDocument()
    })

    it('keeps the team when the deletion is cancelled', async () => {
      session.viewerId = 'u2'
      renderPage()

      fireEvent.click(screen.getByRole('button', { name: 'Eliminar equipo' }))
      const dialog = await screen.findByRole('dialog')
      fireEvent.click(within(dialog).getByRole('button', { name: 'Cancelar' }))

      expect(store.removeTeam).not.toHaveBeenCalled()
    })
  })
})
