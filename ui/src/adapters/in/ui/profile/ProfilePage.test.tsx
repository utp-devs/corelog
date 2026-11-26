import { App } from 'antd'
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { AppError } from '@/core/shared/app-error'

const mocks = vi.hoisted(() => ({
  setState: vi.fn(),
  changePassword: vi.fn(),
  logout: vi.fn(),
  setThemeMode: vi.fn(),
  listSessions: vi.fn(),
  revokeSession: vi.fn(),
  revokeOtherSessions: vi.fn(),
  deactivateAccount: vi.fn(),
  getPreferences: vi.fn(),
  updatePreferences: vi.fn(),
  getMe: vi.fn(),
  updateMe: vi.fn(),
  getAvatar: vi.fn(),
  uploadMyAvatar: vi.fn(),
  deleteMyAvatar: vi.fn(),
  user: {
    id: 'user-1',
    name: 'Ana Torres',
    email: 'ana@corelog.test',
    role: 'user',
    teamId: null,
    displayName: '',
    jobTitle: 'Analista',
    department: 'Operaciones',
    avatarKey: null,
    timezone: 'America/Bogota',
    createdAt: '2026-01-10T00:00:00Z',
    lastLoginAt: null,
    emailVerifiedAt: null,
  },
}))



vi.mock('@/adapters/in/state/auth.store', () => {
  const state = {
    user: mocks.user,
    changePassword: mocks.changePassword,
    logout: mocks.logout,
  }
  const useAuthStore = (selector: (s: unknown) => unknown) => selector(state)
  return { useAuthStore: Object.assign(useAuthStore, { getState: () => state, setState: mocks.setState }) }
})

vi.mock('@/adapters/out/http/people-http-repository', () => ({
  PeopleHttpRepository: class {},
  ProfileHttpRepository: class {
    getMe = mocks.getMe
    updateMe = mocks.updateMe
  },
}))

vi.mock('@/adapters/out/http/avatar-http-repository', () => ({
  AvatarHttpRepository: class {
    getAvatar = mocks.getAvatar
    uploadMyAvatar = mocks.uploadMyAvatar
    deleteMyAvatar = mocks.deleteMyAvatar
  },
}))

vi.mock('@/adapters/out/http/account-security-http-repository', () => ({
  AccountSecurityHttpRepository: class {
    listSessions = mocks.listSessions
    revokeSession = mocks.revokeSession
    revokeOtherSessions = mocks.revokeOtherSessions
    deactivateAccount = mocks.deactivateAccount
  },
}))

vi.mock('@/adapters/out/http/notification-http-repository', () => ({
  NotificationHttpRepository: class {
    getPreferences = mocks.getPreferences
    updatePreferences = mocks.updatePreferences
  },
}))

vi.mock('@/app/theme-context', () => ({
  useThemeMode: () => ({ themeMode: 'dark', setThemeMode: mocks.setThemeMode, toggleTheme: vi.fn() }),
}))

import { ProfilePage } from './ProfilePage'

const sessionsFixture = [
  { id: 'current-1', userAgent: 'Chrome en Windows', createdAt: '2026-11-22T10:00:00Z', lastSeenAt: '2026-11-22T11:00:00Z', current: true },
  { id: 'other-1', userAgent: 'Movil de prueba', createdAt: '2026-11-21T08:00:00Z', lastSeenAt: '2026-11-21T09:00:00Z', current: false },
]

function renderAt(path: string) {
  render(
    <App>
      <MemoryRouter initialEntries={[path]}>
        <Routes>
          <Route path="/profile" element={<ProfilePage />} />
          <Route path="/profile/:section" element={<ProfilePage />} />
        </Routes>
      </MemoryRouter>
    </App>,
  )
}

describe('ProfilePage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.listSessions.mockResolvedValue(sessionsFixture)
    mocks.getPreferences.mockResolvedValue({ inApp: true })
    mocks.updatePreferences.mockImplementation(async (inApp: boolean) => ({ inApp }))
    mocks.revokeSession.mockResolvedValue(undefined)
    mocks.revokeOtherSessions.mockResolvedValue(undefined)
    mocks.deactivateAccount.mockResolvedValue(undefined)
    mocks.updateMe.mockImplementation(async (payload: Record<string, unknown>) => ({ ...mocks.user, ...payload }))
  })

  describe('general section', () => {
    it('shows the profile data with the email read-only and the save button disabled until a change', () => {
      renderAt('/profile/general')

      expect(screen.getByRole('textbox', { name: /^Nombre\b(?! para)/ })).toHaveValue('Ana Torres')
      expect(screen.getByRole('textbox', { name: 'Cargo' })).toHaveValue('Analista')
      expect(screen.getByRole('textbox', { name: 'Correo electrónico' })).toBeDisabled()
      expect(screen.getByText('America/Bogota')).toBeInTheDocument()
      expect(screen.getByRole('button', { name: 'Guardar cambios' })).toBeDisabled()
    })

    it('saves the changed fields and updates the signed-in user', async () => {
      renderAt('/profile/general')

      fireEvent.change(screen.getByRole('textbox', { name: 'Nombre para mostrar' }), { target: { value: 'Ana T.' } })
      fireEvent.click(screen.getByRole('button', { name: 'Guardar cambios' }))

      await waitFor(() => expect(mocks.updateMe).toHaveBeenCalledWith(expect.objectContaining({
        name: 'Ana Torres',
        displayName: 'Ana T.',
        jobTitle: 'Analista',
        timezone: 'America/Bogota',
      })))
      await waitFor(() => expect(mocks.setState).toHaveBeenCalledWith({ user: expect.objectContaining({ displayName: 'Ana T.' }) }))
    })

    it('maps server field errors to the matching field', async () => {
      mocks.updateMe.mockRejectedValueOnce(new AppError({
        status: 422,
        code: 'validation.failed',
        message: 'Revisa los datos',
        details: [{ field: 'job_title', message: 'Cargo demasiado largo' }],
      }))
      renderAt('/profile/general')

      fireEvent.change(screen.getByRole('textbox', { name: 'Cargo' }), { target: { value: 'Otro cargo' } })
      fireEvent.click(screen.getByRole('button', { name: 'Guardar cambios' }))

      expect(await screen.findByText('Cargo demasiado largo')).toBeInTheDocument()
    })

    it('maps the timezone error code to the timezone field', async () => {
      mocks.updateMe.mockRejectedValueOnce(new AppError({
        status: 422,
        code: 'user.timezone_invalid',
        message: 'Zona horaria no válida',
      }))
      renderAt('/profile/general')

      fireEvent.change(screen.getByRole('textbox', { name: 'Cargo' }), { target: { value: 'Otro cargo' } })
      fireEvent.click(screen.getByRole('button', { name: 'Guardar cambios' }))

      expect(await screen.findByText('Zona horaria no válida.')).toBeInTheDocument()
    })

    it('asks before leaving the section with unsaved changes and stays when the user chooses to keep editing', async () => {
      renderAt('/profile/general')

      fireEvent.change(screen.getByRole('textbox', { name: 'Nombre para mostrar' }), { target: { value: 'Ana T.' } })
      fireEvent.click(within(screen.getByRole('navigation', { name: 'Secciones de la cuenta' })).getByText('Notificaciones'))

      const dialog = await screen.findByRole('dialog')
      expect(within(dialog).getAllByText('Tienes cambios sin guardar').length).toBeGreaterThan(0)
      fireEvent.click(within(dialog).getByRole('button', { name: 'Seguir editando' }))

      expect(screen.getByRole('textbox', { name: 'Nombre para mostrar' })).toHaveValue('Ana T.')
      expect(screen.queryByRole('switch', { name: 'Avisos en la aplicación' })).not.toBeInTheDocument()
    })

    it('leaves the section after confirming the discard', async () => {
      renderAt('/profile/general')

      fireEvent.change(screen.getByRole('textbox', { name: 'Nombre para mostrar' }), { target: { value: 'Ana T.' } })
      fireEvent.click(within(screen.getByRole('navigation', { name: 'Secciones de la cuenta' })).getByText('Notificaciones'))
      const dialog = await screen.findByRole('dialog')
      fireEvent.click(within(dialog).getByRole('button', { name: 'Descartar cambios' }))

      expect(await screen.findByRole('switch', { name: 'Avisos en la aplicación' })).toBeInTheDocument()
    })
  })

  describe('other sections (kept from the previous page)', () => {
    it('lists the sessions and closes one that is not the current session', async () => {
      renderAt('/profile/security')
      expect(await screen.findByText('Movil de prueba')).toBeInTheDocument()
      expect(screen.getByText('Esta sesión')).toBeInTheDocument()

      fireEvent.click(screen.getByRole('button', { name: 'Cerrar' }))
      await waitFor(() => expect(mocks.revokeSession).toHaveBeenCalledWith('other-1'))
    })

    it('requires the password before deactivating the account', async () => {
      renderAt('/profile/security')
      fireEvent.click(screen.getByRole('button', { name: /Eliminar cuenta/ }))

      const dialog = await screen.findByRole('dialog')
      const confirm = within(dialog).getByRole('button', { name: /Eliminar cuenta/ })
      expect(confirm).toBeDisabled()

      fireEvent.change(within(dialog).getByLabelText('Contraseña actual'), { target: { value: 'clave-prueba' } })
      expect(confirm).toBeEnabled()
      fireEvent.click(confirm)

      await waitFor(() => expect(mocks.deactivateAccount).toHaveBeenCalledWith('clave-prueba'))
      expect(mocks.logout).toHaveBeenCalled()
    })

    it('saves the in-app notification preference when the switch changes', async () => {
      renderAt('/profile/notifications')
      const toggle = await screen.findByRole('switch', { name: 'Avisos en la aplicación' })
      await waitFor(() => expect(toggle).not.toBeDisabled())

      fireEvent.click(toggle)
      await waitFor(() => expect(mocks.updatePreferences).toHaveBeenCalledWith(false))
    })

    it('changes the theme from the appearance section', async () => {
      renderAt('/profile/appearance')
      fireEvent.click(screen.getByText('Claro'))
      await waitFor(() => expect(mocks.setThemeMode).toHaveBeenCalledWith('light'))
    })

    it('shows the placeholder for tokens', () => {
      renderAt('/profile/tokens')
      expect(screen.getByText('Próximamente')).toBeInTheDocument()
      expect(screen.getByRole('heading', { name: 'Tokens' })).toBeInTheDocument()
    })
  })

  it('shows the account summary with the role label', () => {
    renderAt('/profile')
    expect(screen.getByLabelText('Resumen de la cuenta')).toBeInTheDocument()
    expect(screen.getByText('Usuario')).toBeInTheDocument()
  })
})
