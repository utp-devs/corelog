import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { createOrgAdminStore } from '@/adapters/in/state/org-admin.store'
import { AppError } from '@/core/shared/app-error'
import type { Organization, OrgRole } from '@/core/organizations/domain/organization'
import { createFakeAdminRepository, memberOf, ORG } from '../members/test-fakes'
import { OrgSettingsPage } from './OrgSettingsPage'

const state = vi.hoisted(() => ({
  role: 'owner' as OrgRole,
  org: {
    id: 'org-1',
    slug: 'acme',
    name: 'Acme',
    description: 'Equipo de pruebas',
    settings: {
      whoCanCreateProjects: 'members',
      whoCanCreateTeams: 'admins',
      defaultProjectVisibility: 'open',
      require2fa: false,
      maxUploadBytes: 2 * 1024 * 1024,
      hoursPerDay: 8,
      daysPerWeek: 5,
      timezone: 'Europe/Madrid',
    },
    createdAt: '2026-01-01T00:00:00Z',
    updatedAt: '2026-01-02T00:00:00Z',
  } as Organization,
  fetchOrgs: vi.fn(async () => undefined),
  setOrgs: vi.fn(),
  toastError: vi.fn(),
  toastSuccess: vi.fn(),
}))

vi.mock('@/adapters/in/state/org.store', () => ({
  useCurrentOrg: () => ({ org: { ...state.org, role: state.role }, role: state.role }),
  useOrgStore: {
    getState: () => ({ fetchOrgs: state.fetchOrgs }),
    setState: state.setOrgs,
  },
}))

vi.mock('@/adapters/in/state/auth.store', () => ({
  useAuthStore: (selector: (s: { user: { id: string } }) => unknown) => selector({ user: { id: 'me' } }),
}))

vi.mock('@/shared/ui/Toaster', () => ({
  toast: {
    success: state.toastSuccess,
    error: state.toastError,
    info: vi.fn(),
    undo: vi.fn(),
  },
}))

function renderPage(repo = createFakeAdminRepository()) {
  const store = createOrgAdminStore(repo)
  render(
    <MemoryRouter>
      <OrgSettingsPage store={store} />
    </MemoryRouter>,
  )
  return { repo, store }
}

describe('OrgSettingsPage', () => {
  beforeEach(() => {
    state.role = 'owner'
    vi.clearAllMocks()
  })

  it('lets the owner edit, shows the danger zone and keeps save disabled until something changes', async () => {
    renderPage()
    const save = screen.getByRole('button', { name: 'Guardar cambios' })
    expect(save).toBeDisabled()
    expect(screen.queryByText('Solo lectura')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Transferir propiedad' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Eliminar organización' })).toBeInTheDocument()

    fireEvent.change(screen.getByRole('textbox', { name: /Nombre/ }), { target: { value: 'Acme Labs' } })
    await waitFor(() => expect(save).toBeEnabled())
  })

  it('saves the editable fields and never sends the 2FA flag', async () => {
    const { repo } = renderPage()
    fireEvent.change(screen.getByRole('textbox', { name: /Nombre/ }), { target: { value: 'Acme Labs' } })
    fireEvent.click(screen.getByRole('button', { name: 'Guardar cambios' }))
    await waitFor(() => expect(repo.updateOrganization).toHaveBeenCalledTimes(1))
    const [id, patch] = vi.mocked(repo.updateOrganization).mock.calls[0]
    expect(id).toBe(ORG.id)
    expect(patch).toMatchObject({ name: 'Acme Labs' })
    expect(JSON.stringify(patch)).not.toMatch(/require2fa|require_2fa/)
    await waitFor(() => expect(state.toastSuccess).toHaveBeenCalledWith('Configuración guardada'))
  })

  it('shows the 2FA row as informational with the coming-soon note', () => {
    renderPage()
    expect(screen.getByText('Exigir verificación en dos pasos')).toBeInTheDocument()
    expect(screen.getByText('Disponible próximamente')).toBeInTheDocument()
  })

  it('lets an admin edit settings but not transfer or delete the organization', () => {
    state.role = 'admin'
    renderPage()
    expect(screen.queryByText('Solo lectura')).not.toBeInTheDocument()
    expect(screen.getByRole('textbox', { name: /Nombre/ })).toBeEnabled()
    expect(screen.queryByRole('button', { name: 'Transferir propiedad' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Eliminar organización' })).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Salir de la organización' })).toBeInTheDocument()
  })

  it('shows a read-only banner and disables the form for a member', () => {
    state.role = 'member'
    renderPage()
    expect(screen.getByText('Solo lectura')).toBeInTheDocument()
    expect(screen.getByRole('textbox', { name: /Nombre/ })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Guardar cambios' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Salir de la organización' })).toBeEnabled()
  })

  it('maps server field errors to the form', async () => {
    const repo = createFakeAdminRepository({
      updateOrganization: vi.fn(async () => {
        throw new AppError({
          status: 422,
          code: 'org.settings_invalid',
          message: 'Revisa la configuración',
          details: [{ field: 'settings.hours_per_day', message: 'Debe estar entre 1 y 24' }],
        })
      }),
    })
    renderPage(repo)
    fireEvent.change(screen.getByRole('textbox', { name: /Nombre/ }), { target: { value: 'Acme Labs' } })
    fireEvent.click(screen.getByRole('button', { name: 'Guardar cambios' }))
    expect(await screen.findByText('Debe estar entre 1 y 24')).toBeInTheDocument()
  })

  it('reports the last owner error when leaving the organization', async () => {
    const leave = vi.fn(async () => {
      throw new AppError({ status: 409, code: 'org.last_owner', message: 'Es el último propietario' })
    })
    renderPage(createFakeAdminRepository({ leave }))
    fireEvent.click(screen.getByRole('button', { name: 'Salir de la organización' }))
    await screen.findByText('¿Salir de la organización?')
    const dialog = screen.getByRole('dialog')
    fireEvent.click(within(dialog).getByRole('button', { name: 'Salir' }))
    await waitFor(() => expect(state.toastError).toHaveBeenCalled())
    expect(state.toastError.mock.calls[0][0]).toMatch(/último propietario/)
    expect(leave).toHaveBeenCalledWith(ORG.id)
  })

  it('deletes the organization only after the slug is typed', async () => {
    const deleteOrganization = vi.fn(async () => undefined)
    renderPage(createFakeAdminRepository({ deleteOrganization }))
    fireEvent.click(screen.getByRole('button', { name: 'Eliminar organización' }))
    await screen.findByText('Eliminar la organización', { selector: '.ant-modal-title' })
    const dialog = screen.getByRole('dialog')
    const confirm = within(dialog).getByRole('button', { name: 'Eliminar' })
    expect(confirm).toBeDisabled()
    fireEvent.change(within(dialog).getByLabelText(/Escribe/), { target: { value: 'acme' } })
    fireEvent.click(confirm)
    await waitFor(() => expect(deleteOrganization).toHaveBeenCalledWith(ORG.id, 'acme'))
  })

  it('offers only members as transfer targets, excluding the caller', async () => {
    const repo = createFakeAdminRepository({
      listMembers: vi.fn(async () => [memberOf('me', 'Yo mismo', 'owner'), memberOf('u-2', 'Luis Pardo', 'member')]),
    })
    renderPage(repo)
    fireEvent.mouseDown(await screen.findByRole('combobox', { name: 'Nuevo propietario' }))
    expect(await screen.findByText(/Luis Pardo/)).toBeInTheDocument()
    expect(screen.queryByText(/Yo mismo/)).not.toBeInTheDocument()
  })
})
