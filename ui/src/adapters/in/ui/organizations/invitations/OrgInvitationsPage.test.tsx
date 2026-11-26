import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { createOrgAdminStore } from '@/adapters/in/state/org-admin.store'
import type { OrgRole } from '@/core/organizations/domain/organization'
import {
  createdOf,
  createFakeAdminRepository,
  invitationOf,
  ORG,
  pickSelectOption,
} from '../members/test-fakes'
import { OrgInvitationsPage } from './OrgInvitationsPage'

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

vi.mock('@/shared/ui/Toaster', () => ({
  toast: { success: state.toastSuccess, error: state.toastError, info: vi.fn(), undo: vi.fn() },
}))

function renderPage(repo = createFakeAdminRepository()) {
  const store = createOrgAdminStore(repo)
  render(
    <MemoryRouter>
      <OrgInvitationsPage store={store} />
    </MemoryRouter>,
  )
  return { repo, store }
}

function typeEmails(text: string) {
  fireEvent.change(screen.getByLabelText(/Correos electrónicos/), { target: { value: text } })
}

describe('OrgInvitationsPage', () => {
  beforeEach(() => {
    state.role = 'owner'
    vi.clearAllMocks()
    Object.defineProperty(navigator, 'clipboard', {
      configurable: true,
      value: { writeText: vi.fn(async () => undefined) },
    })
  })

  it('shows the read-only banner and the forbidden state to a member without loading anything', () => {
    state.role = 'member'
    const { repo } = renderPage()
    expect(screen.getByText('Solo lectura')).toBeInTheDocument()
    expect(screen.getByText('No tienes permiso para ver esto')).toBeInTheDocument()
    expect(repo.listInvitations).not.toHaveBeenCalled()
  })

  it('shows the empty state with a call to action when nothing is pending', async () => {
    renderPage()
    expect(await screen.findByText('No hay invitaciones pendientes')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Invitar personas' })).toBeInTheDocument()
  })

  it('rejects an invalid email before calling the API', async () => {
    const { repo } = renderPage()
    typeEmails('ana@acme.test, no-es-correo')
    fireEvent.click(screen.getByRole('button', { name: 'Enviar invitaciones' }))
    expect(await screen.findByText(/No válidos: no-es-correo/)).toBeInTheDocument()
    expect(repo.createInvitations).not.toHaveBeenCalled()
  })

  it('creates one invitation per email and shows the one-time links with copy buttons', async () => {
    const createInvitations = vi.fn(async () => [
      createdOf('i-1', 'ana@acme.test'),
      createdOf('i-2', 'luis@acme.test'),
    ])
    const { repo } = renderPage(createFakeAdminRepository({ createInvitations }))
    typeEmails('ana@acme.test; luis@acme.test')
    fireEvent.click(screen.getByRole('button', { name: 'Enviar invitaciones' }))

    await waitFor(() => expect(repo.createInvitations).toHaveBeenCalledWith(ORG.id, [
      { email: 'ana@acme.test', role: 'member' },
      { email: 'luis@acme.test', role: 'member' },
    ]))
    expect(await screen.findByText('Enlaces de invitación')).toBeInTheDocument()
    expect(screen.getByText(/Este enlace solo se muestra una vez/)).toBeInTheDocument()

    const copy = screen.getByRole('button', { name: 'Copiar enlace de ana@acme.test' })
    fireEvent.click(copy)
    await waitFor(() =>
      expect(navigator.clipboard.writeText).toHaveBeenCalledWith(createdOf('i-1', 'ana@acme.test').acceptUrl),
    )
    await waitFor(() => expect(state.toastSuccess).toHaveBeenCalledWith('Enlace copiado'))
  })

  it('reports a clipboard failure instead of failing silently', async () => {
    Object.defineProperty(navigator, 'clipboard', {
      configurable: true,
      value: { writeText: vi.fn(async () => Promise.reject(new Error('denied'))) },
    })
    const createInvitations = vi.fn(async () => [createdOf('i-1', 'ana@acme.test')])
    renderPage(createFakeAdminRepository({ createInvitations }))
    typeEmails('ana@acme.test')
    fireEvent.click(screen.getByRole('button', { name: 'Enviar invitaciones' }))
    fireEvent.click(await screen.findByRole('button', { name: 'Copiar enlace de ana@acme.test' }))
    await waitFor(() => expect(state.toastError).toHaveBeenCalledWith('No se pudo copiar', expect.any(String)))
  })

  it('switches between pending and all invitations', async () => {
    const { repo } = renderPage(
      createFakeAdminRepository({ listInvitations: vi.fn(async () => [invitationOf('i-1', 'ana@acme.test')]) }),
    )
    expect(await screen.findByText('ana@acme.test')).toBeInTheDocument()
    fireEvent.click(screen.getByText('Todas'))
    await waitFor(() => expect(repo.listInvitations).toHaveBeenLastCalledWith(ORG.id, 'all'))
  })

  it('resends an invitation and shows the new link', async () => {
    const resendInvitation = vi.fn(async () => createdOf('i-1', 'ana@acme.test'))
    const { repo } = renderPage(
      createFakeAdminRepository({
        listInvitations: vi.fn(async () => [invitationOf('i-1', 'ana@acme.test')]),
        resendInvitation,
      }),
    )
    const row = (await screen.findByText('ana@acme.test')).closest('tr') as HTMLElement
    fireEvent.click(within(row).getByRole('button', { name: 'Acciones de la fila' }))
    fireEvent.click(await screen.findByText('Reenviar (nuevo enlace)'))
    await waitFor(() => expect(repo.resendInvitation).toHaveBeenCalledWith(ORG.id, 'i-1'))
    expect(await screen.findByRole('button', { name: 'Copiar enlace de ana@acme.test' })).toBeInTheDocument()
  })

  it('revokes a pending invitation after confirmation', async () => {
    const { repo } = renderPage(
      createFakeAdminRepository({ listInvitations: vi.fn(async () => [invitationOf('i-1', 'ana@acme.test')]) }),
    )
    const row = (await screen.findByText('ana@acme.test')).closest('tr') as HTMLElement
    fireEvent.click(within(row).getByRole('button', { name: 'Acciones de la fila' }))
    fireEvent.click(await screen.findByText('Revocar'))
    const dialog = await screen.findByRole('dialog')
    fireEvent.click(within(dialog).getByRole('button', { name: 'Revocar' }))
    await waitFor(() => expect(repo.revokeInvitation).toHaveBeenCalledWith(ORG.id, 'i-1'))
  })

  it('offers no actions on accepted invitations', async () => {
    renderPage(
      createFakeAdminRepository({
        listInvitations: vi.fn(async () => [invitationOf('i-2', 'marta@acme.test', 'accepted')]),
      }),
    )
    expect(await screen.findByText('marta@acme.test')).toBeInTheDocument()
    expect(screen.getByText('Aceptada')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Acciones de la fila' })).not.toBeInTheDocument()
    expect(screen.queryByText('Revocar')).not.toBeInTheDocument()
    expect(screen.queryByText('Reenviar (nuevo enlace)')).not.toBeInTheDocument()
  })

  it('limits the admin role choices to member and guest', async () => {
    state.role = 'admin'
    renderPage()
    fireEvent.mouseDown(screen.getByRole('combobox', { name: /Rol para todos/ }))
    expect(await screen.findByText('Invitado', { selector: '.ant-select-dropdown:not(.ant-select-dropdown-hidden) *' })).toBeInTheDocument()
    expect(screen.queryByText('Administrador', { selector: '.ant-select-dropdown:not(.ant-select-dropdown-hidden) *' })).not.toBeInTheDocument()
  })

  it('invites with the chosen role', async () => {
    const { repo } = renderPage()
    await pickSelectOption(screen.getByRole('combobox', { name: /Rol para todos/ }), 'Invitado')
    typeEmails('marta@acme.test')
    fireEvent.click(screen.getByRole('button', { name: 'Enviar invitaciones' }))
    await waitFor(() =>
      expect(repo.createInvitations).toHaveBeenCalledWith(ORG.id, [{ email: 'marta@acme.test', role: 'guest' }]),
    )
  })
})
