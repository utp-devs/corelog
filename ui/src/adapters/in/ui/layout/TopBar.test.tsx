import { fireEvent, render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { useAuthStore } from '@/adapters/in/state/auth.store'
import { ThemeContext } from '@/app/theme-context'
import type { User } from '@/core/auth/domain/user'
import { closePalette, getPaletteState } from '@/shared/ui/CommandPalette/paletteState'

import { TopBar } from './TopBar'

vi.mock('./NotificationsBell', () => ({
  NotificationsBell: () => <button type="button" aria-label="Notificaciones">campana</button>,
}))

const base: User = {
  id: 'u1',
  name: 'Luis Ruiz',
  email: 'luis@example.com',
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

  it('shows the primary menus and Equipos to every user', () => {
    renderBar()
    const nav = screen.getByRole('navigation', { name: 'Principal' })
    expect(nav).toHaveTextContent('Proyectos')
    expect(nav).toHaveTextContent('Filtros')
    expect(nav).toHaveTextContent('Paneles')
    expect(nav).toHaveTextContent('Equipos')
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

/** Minimal matchMedia: "(max-width: Npx)" matches when the viewport is at most N px wide. */
function setViewport(width: number) {
  window.matchMedia = ((query: string) => {
    const max = /max-width:\s*([\d.]+)px/.exec(query)
    return {
      matches: max ? width <= Number(max[1]) : false,
      media: query,
      onchange: null,
      addListener: () => undefined,
      removeListener: () => undefined,
      addEventListener: () => undefined,
      removeEventListener: () => undefined,
      dispatchEvent: () => false,
    }
  }) as typeof window.matchMedia
}

describe('TopBar responsive layout', () => {
  beforeEach(() => {
    useAuthStore.setState({ user: base, status: 'authenticated', sessionChecked: true })
  })

  afterEach(() => {
    // jsdom has no matchMedia by default; restore that state for the other suites.
    delete (window as Partial<Window>).matchMedia
  })

  it('at 1280 px shows the full bar: menus, text Create, search field and notes', () => {
    setViewport(1280)
    renderBar({ onOpenMenu: () => undefined, org: { id: 'o1', name: 'Acme', slug: 'acme' }, orgSwitcherSlot: <span>Switcher real</span> })
    expect(screen.getByRole('navigation', { name: 'Principal' })).toHaveTextContent('Equipos')
    expect(screen.getByRole('button', { name: /Crear/ })).toHaveTextContent('Crear')
    expect(screen.getByLabelText('Buscar proyectos e incidencias')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Notas (próximamente)' })).toBeInTheDocument()
    expect(screen.getByText('Switcher real')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Buscar' })).not.toBeInTheDocument()
  })

  it('at 1100 px keeps the menus but turns the search field into an icon', () => {
    setViewport(1100)
    renderBar()
    expect(screen.getByRole('navigation', { name: 'Principal' })).toBeInTheDocument()
    expect(screen.queryByLabelText('Buscar proyectos e incidencias')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Buscar' })).toBeInTheDocument()
  })

  it('at 820 px collapses the menus into the Menú button and keeps Create, org switcher, search icon and avatar', () => {
    setViewport(820)
    renderBar({ onOpenMenu: () => undefined, org: { id: 'o1', name: 'Acme', slug: 'acme' }, orgSwitcherSlot: <span>Switcher real</span> })
    expect(screen.queryByRole('navigation', { name: 'Principal' })).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Abrir menú de navegación' })).toHaveTextContent('Menú')
    expect(screen.getByRole('button', { name: /Crear/ })).toHaveTextContent('Crear')
    expect(screen.getByText('Switcher real')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Buscar' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Menú de Luis Ruiz/ })).toBeInTheDocument()
  })

  it('at 390 px shows icon-only controls, the logo icon and no org switcher or notes in the bar', () => {
    setViewport(390)
    renderBar({ onOpenMenu: () => undefined, org: { id: 'o1', name: 'Acme', slug: 'acme' }, orgSwitcherSlot: <span>Switcher real</span> })
    const create = screen.getByRole('button', { name: /Crear/ })
    expect(create).not.toHaveTextContent(/Crear/)
    expect(create.querySelector('.anticon-plus')).not.toBeNull()
    expect(screen.getByRole('button', { name: 'Abrir menú de navegación' })).not.toHaveTextContent('Menú')
    expect(screen.getByRole('button', { name: 'Buscar' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Menú de Luis Ruiz/ })).toBeInTheDocument()
    expect(screen.queryByText('Switcher real')).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Notas (próximamente)' })).not.toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'CoreLog, ir a incidencias' }).querySelector('img')).toHaveAttribute('src', '/brand/corelog-icon-64.png')
  })

  it('opens the command palette from the icon search button', () => {
    setViewport(820)
    renderBar()
    fireEvent.click(screen.getByRole('button', { name: 'Buscar' }))
    expect(getPaletteState().open).toBe(true)
    closePalette()
  })
})
