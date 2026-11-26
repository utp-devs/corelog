import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { createOrgAdminStore } from '@/adapters/in/state/org-admin.store'
import { AppError } from '@/core/shared/app-error'
import type { OrgRole } from '@/core/organizations/domain/organization'
import { createFakeAdminRepository, memberOf, ME_ID, ORG, pickSelectOption } from './test-fakes'
import { OrgMembersPage } from './OrgMembersPage'

const state = vi.hoisted(() => ({
  role: 'owner' as OrgRole,
  org: {
    id: 'org-1',
    slug: 'acme',
    name: 'Acme',
    description: '',
    settings: {
      whoCanCreateProjects: 'members',
      whoCanCreateTeams: 'admins',
      defaultProjectVisibility: 'open',
      require2fa: false,
      maxUploadBytes: 2097152,
      hoursPerDay: 8,
      daysPerWeek: 5,
      timezone: 'UTC',
    },
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-02T00:00:00Z',
  },
  toastError: vi.fn(),
  toastSuccess: vi.fn(),
}))

vi.mock('@/adapters/in/state/org.store', () => ({
  useCurrentOrg: () => ({ org: state.org, role: state.role }),
}))

vi.mock('@/adapters/in/state/auth.store', () => ({
  useAuthStore: (selector: (s: { user: { id: string } }) => unknown) => selector({ user: { id: 'me' } }),
}))

vi.mock('@/shared/ui/Toaster', () => ({
  toast: { success: state.toastSuccess, error: state.toastError, info: vi.fn(), undo: vi.fn() },
}))

const team = () => [
  memberOf(ME_ID, 'Yo Mismo', 'owner'),
  memberOf('u-2', 'Luis Pardo', 'member'),
  memberOf('u-3', 'Marta Ruiz', 'guest'),
  memberOf('u-4', 'Carla Vega', 'admin'),
]

function renderPage(repo = createFakeAdminRepository({ listMembers: vi.fn(async () => team()) })) {
  const store = createOrgAdminStore(repo)
  render(
    <MemoryRouter>
      <OrgMembersPage store={store} />
    </MemoryRouter>,
  )
  return { repo, store }
}

describe('OrgMembersPage', () => {
  beforeEach(() => {
    state.role = 'owner'
    vi.clearAllMocks()
  })

  it('shows every role select for an owner except on their own row', async () => {
    renderPage()
    expect(await screen.findByText('Luis Pardo')).toBeInTheDocument()
    expect(screen.getByRole('combobox', { name: 'Rol de Luis Pardo' })).toBeInTheDocument()
    expect(screen.getByRole('combobox', { name: 'Rol de Carla Vega' })).toBeInTheDocument()
    expect(screen.queryByRole('combobox', { name: 'Rol de Yo Mismo' })).not.toBeInTheDocument()
    expect(screen.getByText('(tú)')).toBeInTheDocument()
    expect(screen.queryByText('Solo lectura')).not.toBeInTheDocument()
    expect(document.querySelector('[data-page="org-members"]')).toHaveAttribute('data-fill', 'true')
  })

  it('limits an admin to changing members and guests', async () => {
    state.role = 'admin'
    renderPage()
    expect(await screen.findByText('Luis Pardo')).toBeInTheDocument()
    expect(screen.getByRole('combobox', { name: 'Rol de Luis Pardo' })).toBeInTheDocument()
    expect(screen.getByRole('combobox', { name: 'Rol de Marta Ruiz' })).toBeInTheDocument()
    expect(screen.queryByRole('combobox', { name: 'Rol de Carla Vega' })).not.toBeInTheDocument()
    expect(screen.queryByRole('combobox', { name: 'Rol de Yo Mismo' })).not.toBeInTheDocument()
  })

  it('shows a read-only banner and no controls for a member', async () => {
    state.role = 'member'
    renderPage()
    expect(await screen.findByText('Luis Pardo')).toBeInTheDocument()
    expect(screen.getByText('Solo lectura')).toBeInTheDocument()
    expect(screen.queryByRole('combobox', { name: /^Rol de/ })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Invitar personas' })).not.toBeInTheDocument()
  })

  it('shows the forbidden state for a guest and does not load the list', () => {
    state.role = 'guest'
    const { repo } = renderPage()
    expect(screen.getByText('No tienes permiso para ver esto')).toBeInTheDocument()
    expect(repo.listMembers).not.toHaveBeenCalled()
  })

  it('confirms a role change before calling the API', async () => {
    const { repo } = renderPage()
    await pickSelectOption(await screen.findByRole('combobox', { name: 'Rol de Luis Pardo' }), 'Administrador')

    await screen.findByText(/¿Cambiar el rol de Luis Pardo de Miembro a Administrador\?/)
    expect(repo.changeMemberRole).not.toHaveBeenCalled()

    fireEvent.click(screen.getByRole('button', { name: 'Cambiar rol' }))
    await waitFor(() => expect(repo.changeMemberRole).toHaveBeenCalledWith(ORG.id, 'u-2', 'admin'))
    await waitFor(() => expect(state.toastSuccess).toHaveBeenCalledWith('Rol de Luis Pardo actualizado'))
  })

  it('does not call the API when the role confirmation is cancelled', async () => {
    const { repo } = renderPage()
    await pickSelectOption(await screen.findByRole('combobox', { name: 'Rol de Luis Pardo' }), 'Invitado')
    await screen.findByText(/¿Cambiar el rol/)
    fireEvent.click(screen.getByRole('button', { name: 'Cancelar' }))
    expect(repo.changeMemberRole).not.toHaveBeenCalled()
  })

  it('shows the last owner explanation when the API refuses the change', async () => {
    const repo = createFakeAdminRepository({
      listMembers: vi.fn(async () => team()),
      changeMemberRole: vi.fn(async () => {
        throw new AppError({ status: 409, code: 'org.last_owner', message: 'Es el último propietario' })
      }),
    })
    renderPage(repo)
    await pickSelectOption(await screen.findByRole('combobox', { name: 'Rol de Luis Pardo' }), 'Administrador')
    await screen.findByText(/¿Cambiar el rol de Luis Pardo/)
    fireEvent.click(screen.getByRole('button', { name: 'Cambiar rol' }))
    const alert = await screen.findByRole('alert')
    expect(within(alert).getByText(/Es el último propietario de la organización/)).toBeInTheDocument()
  })

  it('removes one member after typing the email in the danger confirmation', async () => {
    const { repo } = renderPage()
    const row = (await screen.findByText('Luis Pardo')).closest('tr') as HTMLElement
    fireEvent.click(within(row).getByRole('button', { name: 'Acciones de la fila' }))
    fireEvent.click(await screen.findByText('Quitar de la organización'))

    const dialog = await screen.findByRole('dialog')
    const confirm = within(dialog).getByRole('button', { name: 'Quitar' })
    expect(confirm).toBeDisabled()
    fireEvent.change(within(dialog).getByLabelText(/Escribe/), { target: { value: 'u-2@acme.test' } })
    fireEvent.click(confirm)
    await waitFor(() => expect(repo.removeMember).toHaveBeenCalledWith(ORG.id, 'u-2'))
  })

  it('filters by role and by search text', async () => {
    renderPage()
    expect(await screen.findByText('Marta Ruiz')).toBeInTheDocument()
    await pickSelectOption(screen.getByRole('combobox', { name: 'Filtrar por rol' }), 'Invitado')
    await waitFor(() => expect(screen.queryByText('Luis Pardo')).not.toBeInTheDocument())
    expect(screen.getByText('Marta Ruiz')).toBeInTheDocument()
  })

  it('shows the empty state with an invitation call to action when there are no members', async () => {
    renderPage(createFakeAdminRepository({ listMembers: vi.fn(async () => []) }))
    expect(await screen.findByText('Aún no hay miembros')).toBeInTheDocument()
    expect(screen.getAllByRole('button', { name: 'Invitar personas' }).length).toBeGreaterThan(0)
  })

  it('shows the load error with a retry that reloads the list', async () => {
    const listMembers = vi
      .fn<() => Promise<ReturnType<typeof team>>>()
      .mockRejectedValueOnce(new AppError({ status: 500, message: 'Fallo del servidor' }))
      .mockResolvedValue(team())
    renderPage(createFakeAdminRepository({ listMembers }))
    expect(await screen.findByText('Fallo del servidor')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: /Reintentar/ }))
    expect(await screen.findByText('Luis Pardo')).toBeInTheDocument()
    expect(listMembers).toHaveBeenCalledTimes(2)
  })
})
