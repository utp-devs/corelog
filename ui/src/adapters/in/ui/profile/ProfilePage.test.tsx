import { App } from 'antd'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { setThemeMode } = vi.hoisted(() => ({ setThemeMode: vi.fn() }))

vi.mock('@/adapters/in/state/auth.store', () => ({
  useAuthStore: (selector: (state: unknown) => unknown) =>
    selector({
      user: { id: 'user-1', name: 'Ana Torres', email: 'ana@corelog.test', role: 'agent' },
      updateProfile: vi.fn(),
      changePassword: vi.fn(),
    }),
}))

vi.mock('@/app/theme-context', () => ({
  useThemeMode: () => ({ themeMode: 'dark', setThemeMode, toggleTheme: vi.fn() }),
}))

import { ProfilePage } from './ProfilePage'

function renderPage() {
  render(
    <App>
      <ProfilePage />
    </App>,
  )
}

describe('ProfilePage', () => {
  beforeEach(() => {
    setThemeMode.mockReset()
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
    await waitFor(() => expect(setThemeMode).toHaveBeenCalledWith('light'))
  })
})
