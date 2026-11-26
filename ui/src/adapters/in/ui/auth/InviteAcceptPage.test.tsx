import { App } from 'antd'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { useAuthStore } from '@/adapters/in/state/auth.store'
import { useOrgStore } from '@/adapters/in/state/org.store'
import { OrganizationHttpRepository } from '@/adapters/out/http/organization-http-repository'
import type { User } from '@/core/auth/domain/user'
import type { InvitationPreview } from '@/core/organizations/domain/organization'
import { AppError } from '@/core/shared/app-error'

import { InviteAcceptPage } from './InviteAcceptPage'

const TOKEN = 'tok-abcdefghijklmnop'

const user: User = {
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

function preview(patch: Partial<InvitationPreview> = {}): InvitationPreview {
  return {
    organizationName: 'Acme',
    role: 'member',
    email: 'ana@example.com',
    status: 'pending',
    expiresAt: '2026-12-31T00:00:00Z',
    ...patch,
  }
}

function renderPage() {
  return render(
    <App>
    <MemoryRouter initialEntries={[`/invite/${TOKEN}`]}>
      <Routes>
        <Route path="/login" element={<p>Pantalla de acceso</p>} />
        <Route path="/register" element={<p>Pantalla de registro</p>} />
        <Route path="/projects" element={<p>Proyectos</p>} />
        <Route path="/invite/:token" element={<InviteAcceptPage />} />
      </Routes>
    </MemoryRouter>
    </App>,
  )
}

function setSession(current: User | null) {
  useAuthStore.setState({
    user: current,
    status: current ? 'authenticated' : 'idle',
    sessionChecked: true,
  })
}

describe('InviteAcceptPage', () => {
  beforeEach(() => {
    localStorage.clear()
    useOrgStore.getState().reset()
    setSession(user)
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('offers login and register to a visitor and returns to the invitation after auth', async () => {
    setSession(null)
    vi.spyOn(OrganizationHttpRepository.prototype, 'previewInvitation').mockResolvedValue(preview())
    renderPage()
    expect(await screen.findByRole('heading', { name: 'Invitación a Acme' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Aceptar invitación' })).not.toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Iniciar sesión' }))
    expect(await screen.findByText('Pantalla de acceso')).toBeInTheDocument()
  })

  it('shows the organization, role and invited email for a valid invitation', async () => {
    vi.spyOn(OrganizationHttpRepository.prototype, 'previewInvitation').mockResolvedValue(preview())
    renderPage()
    const details = (await screen.findByText(/Correo invitado:/)).parentElement
    expect(details).toHaveTextContent('Rol: Miembro')
    expect(details).toHaveTextContent('Correo invitado: ana@example.com')
    expect(screen.getByRole('button', { name: 'Aceptar invitación' })).toBeEnabled()
  })

  it('accepts, refreshes the organizations, selects the new one and opens projects', async () => {
    vi.spyOn(OrganizationHttpRepository.prototype, 'previewInvitation').mockResolvedValue(preview())
    const accept = vi.spyOn(OrganizationHttpRepository.prototype, 'acceptInvitation').mockResolvedValue({
      organizationId: 'org-acme',
      userId: 'u1',
      role: 'member',
      joinedAt: '2026-01-02T00:00:00Z',
    })
    const list = vi.spyOn(OrganizationHttpRepository.prototype, 'list').mockResolvedValue([])
    renderPage()
    fireEvent.click(await screen.findByRole('button', { name: 'Aceptar invitación' }))
    expect(await screen.findByText('Proyectos')).toBeInTheDocument()
    expect(accept).toHaveBeenCalledWith(TOKEN)
    expect(list).toHaveBeenCalled()
    expect(useOrgStore.getState().currentOrgId).toBe('org-acme')
  })

  it('explains an email mismatch returned by the server and offers to switch accounts', async () => {
    vi.spyOn(OrganizationHttpRepository.prototype, 'previewInvitation').mockResolvedValue(preview({ email: 'otra@example.com' }))
    vi.spyOn(OrganizationHttpRepository.prototype, 'acceptInvitation').mockRejectedValue(
      new AppError({ status: 403, code: 'org.email_mismatch', message: 'mismatch' }),
    )
    renderPage()
    fireEvent.click(await screen.findByRole('button', { name: 'Aceptar invitación' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('Esta invitación es para otra@example.com')
    expect(screen.getByRole('button', { name: 'Cambiar de cuenta' })).toBeInTheDocument()
  })

  it('warns before accepting when the signed-in account is not the invited one', async () => {
    vi.spyOn(OrganizationHttpRepository.prototype, 'previewInvitation').mockResolvedValue(preview({ email: 'otra@example.com' }))
    renderPage()
    expect(await screen.findByText(/has iniciado sesión como ana@example.com/)).toBeInTheDocument()
  })

  it.each([
    ['accepted', /ya se ha utilizado/],
    ['revoked', /fue revocada/],
    ['expired', /ha caducado/],
  ] as const)('shows the %s state without an accept button', async (status, copy) => {
    vi.spyOn(OrganizationHttpRepository.prototype, 'previewInvitation').mockResolvedValue(preview({ status }))
    renderPage()
    expect(await screen.findByText(copy)).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Aceptar invitación' })).not.toBeInTheDocument()
  })

  it('shows the invalid state for an unknown token', async () => {
    vi.spyOn(OrganizationHttpRepository.prototype, 'previewInvitation').mockRejectedValue(
      new AppError({ status: 404, code: 'org.invitation_not_found', message: 'no' }),
    )
    renderPage()
    expect(await screen.findByText('Este enlace no es válido.')).toBeInTheDocument()
    await waitFor(() => expect(screen.queryByRole('button', { name: 'Aceptar invitación' })).not.toBeInTheDocument())
  })
})
