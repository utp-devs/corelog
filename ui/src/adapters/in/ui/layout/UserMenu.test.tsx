import { render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { useAuthStore } from '@/adapters/in/state/auth.store'
import { ThemeContext, type ThemeContextValue } from '@/app/theme-context'
import type { User } from '@/core/auth/domain/user'

import { UserMenu } from './UserMenu'

const mocks = vi.hoisted(() => ({ getAvatar: vi.fn() }))

vi.mock('@/adapters/out/http/avatar-http-repository', () => ({
  AvatarHttpRepository: class {
    getAvatar = mocks.getAvatar
  },
}))

const user: User = {
  id: 'u1',
  name: 'Ana Pérez',
  email: 'ana@example.com',
  role: 'user',
  teamId: null,
  displayName: '',
  jobTitle: '',
  department: '',
  avatarKey: null,
  timezone: 'UTC',
  lastLoginAt: null,
  emailVerifiedAt: null,
  createdAt: '2026-01-01T00:00:00Z',
}

const theme: ThemeContextValue = { themeMode: 'light', setThemeMode: () => undefined, toggleTheme: () => undefined }

function renderMenu() {
  return render(
    <ThemeContext.Provider value={theme}>
      <MemoryRouter>
        <UserMenu />
      </MemoryRouter>
    </ThemeContext.Provider>,
  )
}

describe('UserMenu avatar', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    window.localStorage.clear()
    URL.createObjectURL = vi.fn(() => 'blob:ana-avatar')
    URL.revokeObjectURL = vi.fn()
    mocks.getAvatar.mockResolvedValue(new Blob(['png'], { type: 'image/png' }))
  })

  it('shows the initials of the current user when there is no photo', () => {
    useAuthStore.setState({ user, status: 'authenticated', sessionChecked: true })
    renderMenu()
    const avatar = screen.getByRole('img', { name: 'Ana Pérez' })
    expect(avatar).toHaveTextContent('AP')
    expect(mocks.getAvatar).not.toHaveBeenCalled()
  })

  it('loads the photo with the session when the user has an avatar', async () => {
    useAuthStore.setState({ user: { ...user, avatarKey: 'k1' }, status: 'authenticated', sessionChecked: true })
    renderMenu()
    await waitFor(() => expect(document.querySelector('img[src="blob:ana-avatar"]')).not.toBeNull())
    expect(screen.getByRole('img', { name: 'Ana Pérez' }).tagName).toBe('IMG')
    expect(mocks.getAvatar).toHaveBeenCalledWith('u1')
  })
})
