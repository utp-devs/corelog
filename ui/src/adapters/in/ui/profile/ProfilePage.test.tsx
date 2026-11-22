import { App } from 'antd'
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const mocks = vi.hoisted(() => ({
  setThemeMode: vi.fn(),
  logout: vi.fn(),
  listSessions: vi.fn(),
  revokeSession: vi.fn(),
  revokeOtherSessions: vi.fn(),
  deactivateAccount: vi.fn(),
  getPreferences: vi.fn(),
  updatePreferences: vi.fn(),
}))

vi.mock('@/adapters/in/state/auth.store', () => {
  const state = {
    user: { id: 'user-1', name: 'Ana Torres', email: 'ana@corelog.test', role: 'agent' },
    updateProfile: vi.fn(),
    changePassword: vi.fn(),
    logout: mocks.logout,
  }
  const useAuthStore = (selector: (s: unknown) => unknown) => selector(state)
  return { useAuthStore: Object.assign(useAuthStore, { getState: () => state }) }
})

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

function renderPage() {
  render(
    <App>
      <MemoryRouter>
        <ProfilePage />
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
  })

  it('shows the personal data, password and appearance cards', () => {
    renderPage()
    expect(screen.getByText('Datos personales')).toBeInTheDocument()
    expect(screen.getByText('Contraseña')).toBeInTheDocument()
    expect(screen.getByText('Apariencia')).toBeInTheDocument()
  })

  it('prefills the profile form with the current user', () => {
    renderPage()
    expect(screen.getByDisplayValue('Ana Torres')).toBeInTheDocument()
    expect(screen.getByDisplayValue('ana@corelog.test')).toBeInTheDocument()
  })

  it('changes the theme from the appearance selector', async () => {
    renderPage()
    fireEvent.click(screen.getByText('Claro'))
    await waitFor(() => expect(mocks.setThemeMode).toHaveBeenCalledWith('light'))
  })

  it('lists the sessions and closes one that is not the current session', async () => {
    renderPage()
    expect(await screen.findByText('Movil de prueba')).toBeInTheDocument()
    expect(screen.getByText('Esta sesión')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Cerrar' }))
    await waitFor(() => expect(mocks.revokeSession).toHaveBeenCalledWith('other-1'))
  })

  it('saves the in-app notification preference when the switch changes', async () => {
    renderPage()
    const toggle = await screen.findByRole('switch', { name: 'Avisos en la aplicación' })
    await waitFor(() => expect(toggle).not.toBeDisabled())

    fireEvent.click(toggle)
    await waitFor(() => expect(mocks.updatePreferences).toHaveBeenCalledWith(false))
  })

  it('requires the password before deactivating the account', async () => {
    renderPage()
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
})
