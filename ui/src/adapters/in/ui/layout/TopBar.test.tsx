import { fireEvent, render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { useAuthStore } from '@/adapters/in/state/auth.store'
import { ThemeContext } from '@/app/theme-context'
import type { User } from '@/core/auth/domain/user'

import { TopBar } from './TopBar'

vi.mock('./NotificationsBell', () => ({
  NotificationsBell: () => <button type="button" aria-label="Notificaciones">campana</button>,
}))

const base: User = {
  id: 'u1',
  name: 'Luis Ruiz',
  email: 'luis@example.com',
  role: 'requester',
  teamId: null,
  createdAt: '2026-01-01T00:00:00Z',
}

function renderBar(props: Partial<Parameters<typeof TopBar>[0]> = {}) {
  return render(
    <ThemeContext.Provider value={{ themeMode: 'light', setThemeMode: () => undefined, toggleTheme: () => undefined }}>
      <MemoryRouter>
        <TopBar onOpenShortcuts={() => undefined} {...props} />
      </MemoryRouter>
    </ThemeContext.Provider>,
  )
}

describe('TopBar', () => {
  beforeEach(() => {
    useAuthStore.setState({ user: base, status: 'authenticated', sessionChecked: true })
  })

  it('shows the primary menus and hides Equipos for a requester', () => {
    renderBar()
    const nav = screen.getByRole('navigation', { name: 'Principal' })
    expect(nav).toHaveTextContent('Proyectos')
    expect(nav).toHaveTextContent('Filtros')
    expect(nav).toHaveTextContent('Paneles')
    expect(nav).not.toHaveTextContent('Equipos')
  })

  it('shows Equipos for an admin', () => {
    useAuthStore.setState({ user: { ...base, role: 'admin' } })
    renderBar()
    expect(screen.getByRole('navigation', { name: 'Principal' })).toHaveTextContent('Equipos')
  })

  it('opens the placeholder create modal from the Crear button', () => {
    renderBar()
    fireEvent.click(screen.getByRole('button', { name: /Crear/ }))
    expect(screen.getByRole('dialog', { name: 'Crear incidencia' })).toBeInTheDocument()
  })

  it('renders the notes button disabled', () => {
    renderBar()
    expect(screen.getByRole('button', { name: 'Notas (próximamente)' })).toBeDisabled()
  })

  it('renders the real org switcher slot instead of the organization name', () => {
    renderBar({ org: { id: 'o1', name: 'Acme', slug: 'acme' }, orgSwitcherSlot: <span>Switcher real</span> })
    expect(screen.getByText('Switcher real')).toBeInTheDocument()
    expect(screen.queryByText('Acme')).not.toBeInTheDocument()
  })

  it('shows the menu button only when a handler is provided', () => {
    const { unmount } = renderBar()
    expect(screen.queryByRole('button', { name: 'Abrir menú de navegación' })).not.toBeInTheDocument()
    unmount()
    renderBar({ onOpenMenu: () => undefined })
    expect(screen.getByRole('button', { name: 'Abrir menú de navegación' })).toBeInTheDocument()
  })
})
