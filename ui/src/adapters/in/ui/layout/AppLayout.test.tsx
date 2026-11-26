import { fireEvent, render, screen, within } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { useAuthStore } from '@/adapters/in/state/auth.store'
import { useProjectsStore } from '@/adapters/in/state/projects.store'
import { ThemeContext, type ThemeContextValue } from '@/app/theme-context'
import type { User } from '@/core/auth/domain/user'

import { AppLayout } from './AppLayout'
import { DENSITY_STORAGE_KEY, SIDEBAR_STORAGE_KEY } from './preferences'

vi.mock('./NotificationsBell', () => ({
  NotificationsBell: () => <button type="button" aria-label="Notificaciones">campana</button>,
}))

const user: User = {
  id: 'u1',
  name: 'Ana Pérez',
  email: 'ana@example.com',
  role: 'admin',
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

/** Minimal matchMedia: "(max-width: Npx)" matches when the viewport is at most N px wide. */
function setViewport(width: number) {
  window.matchMedia = ((query: string) => {
    const max = /max-width:\s*([\d.]+)px/.exec(query)
    const matches = max ? width <= Number(max[1]) : false
    return {
      matches,
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

function themeValue(overrides: Partial<ThemeContextValue> = {}): ThemeContextValue {
  return { themeMode: 'light', setThemeMode: () => undefined, toggleTheme: () => undefined, ...overrides }
}

function renderShell(path: string, theme: ThemeContextValue = themeValue()) {
  return render(
    <ThemeContext.Provider value={theme}>
      <MemoryRouter initialEntries={[path]}>
        <Routes>
          <Route path="/login" element={<p>Pantalla de acceso</p>} />
          <Route element={<AppLayout />}>
            <Route path="/issues" element={<p>Lista de incidencias</p>} />
            <Route path="/projects/:id/board" element={<p>Tablero del proyecto</p>} />
            <Route path="/profile" element={<p>Perfil</p>} />
          </Route>
        </Routes>
      </MemoryRouter>
    </ThemeContext.Provider>,
  )
}

const demoProject = {
  id: 'p1',
  key: 'DEM',
  name: 'Proyecto demo',
  description: '',
  type: 'scrum',
  leadId: 'u1',
  visibility: 'private',
  archivedAt: null,
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-01-01T00:00:00Z',
} as unknown as ReturnType<typeof useProjectsStore.getState>['projects'][number]

describe('AppLayout', () => {
  beforeEach(() => {
    window.localStorage.clear()
    setViewport(1440)
    useAuthStore.setState({ user, status: 'authenticated', sessionChecked: true })
    useProjectsStore.setState({ projects: [demoProject] })
  })

  afterEach(() => {
    window.localStorage.clear()
  })

  it('puts the skip link first in the tab order and it moves focus to main', () => {
    renderShell('/issues')
    const focusable = Array.from(document.querySelectorAll<HTMLElement>('a[href], button, input, [tabindex]'))
      .filter((el) => el.tabIndex >= 0)
    expect(focusable[0]).toHaveTextContent('Saltar al contenido')

    fireEvent.click(screen.getByRole('link', { name: 'Saltar al contenido' }))
    expect(document.getElementById('main-content')).toHaveFocus()
  })

  it('renders the landmarks: banner, main navigation, main and the context aside', () => {
    renderShell('/projects/p1/board')
    expect(screen.getByRole('banner')).toBeInTheDocument()
    expect(screen.getByRole('navigation', { name: 'Principal' })).toBeInTheDocument()
    expect(screen.getByRole('main')).toHaveAttribute('id', 'main-content')
    expect(screen.getByRole('complementary', { name: 'Proyecto' })).toBeInTheDocument()
    expect(screen.getByText('Tablero del proyecto')).toBeInTheDocument()
  })

  it('shows the project header and marks the active item with aria-current', () => {
    renderShell('/projects/p1/board')
    const aside = screen.getByRole('complementary', { name: 'Proyecto' })
    expect(within(aside).getByText('Proyecto demo')).toBeInTheDocument()
    expect(within(aside).getByText('DEM')).toBeInTheDocument()
    expect(within(aside).getByRole('link', { name: 'Tablero' })).toHaveAttribute('aria-current', 'page')
    expect(within(aside).getByRole('link', { name: 'Permisos' })).not.toHaveAttribute('aria-current')
  })

  it('has no sidebar in the global context', () => {
    renderShell('/issues')
    expect(screen.queryByRole('complementary')).not.toBeInTheDocument()
  })

  it('collapses below 1280 px by default and persists a manual choice', () => {
    setViewport(1100)
    const { unmount } = renderShell('/projects/p1/board')
    const aside = screen.getByRole('complementary', { name: 'Proyecto' })
    expect(aside).toHaveAttribute('data-collapsed', 'true')

    fireEvent.click(screen.getByRole('button', { name: 'Expandir menú lateral' }))
    expect(window.localStorage.getItem(SIDEBAR_STORAGE_KEY)).toBe('expanded')
    unmount()

    renderShell('/projects/p1/board')
    expect(screen.getByRole('complementary', { name: 'Proyecto' })).toHaveAttribute('data-collapsed', 'false')
  })

  it('replaces the sidebar with a drawer on mobile and opens it from the top bar', () => {
    setViewport(390)
    renderShell('/projects/p1/board')
    expect(screen.queryByRole('complementary')).not.toBeInTheDocument()
    expect(screen.queryByRole('navigation', { name: 'Principal' })).not.toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Abrir menú de navegación' }))
    expect(screen.getByRole('navigation', { name: 'Navegación móvil' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Tablero' })).toBeInTheDocument()
  })

  it('opens the shortcuts dialog from the help menu', () => {
    renderShell('/issues')
    fireEvent.click(screen.getByRole('button', { name: 'Ayuda' }))
    fireEvent.click(screen.getByText('Atajos de teclado'))
    expect(screen.getByRole('dialog', { name: 'Atajos de teclado' })).toBeInTheDocument()
  })
})

describe('UserMenu inside AppLayout', () => {
  beforeEach(() => {
    window.localStorage.clear()
    setViewport(1440)
    useAuthStore.setState({ user, status: 'authenticated', sessionChecked: true })
  })

  it('calls the store logout and goes to /login', () => {
    const logout = vi.fn()
    useAuthStore.setState({ logout })
    renderShell('/issues')
    fireEvent.click(screen.getByRole('button', { name: 'Menú de Ana Pérez' }))
    fireEvent.click(screen.getByText('Cerrar sesión'))
    expect(logout).toHaveBeenCalledTimes(1)
    expect(screen.getByText('Pantalla de acceso')).toBeInTheDocument()
  })

  it('calls the theme toggle from the user menu', () => {
    const toggleTheme = vi.fn()
    renderShell('/issues', themeValue({ toggleTheme }))
    fireEvent.click(screen.getByRole('button', { name: 'Menú de Ana Pérez' }))
    fireEvent.click(screen.getByText('Usar tema oscuro'))
    expect(toggleTheme).toHaveBeenCalledTimes(1)
  })

  it('persists the density choice in corelog.density', () => {
    renderShell('/issues')
    fireEvent.click(screen.getByRole('button', { name: 'Menú de Ana Pérez' }))
    fireEvent.click(screen.getByText('Densidad compacta'))
    expect(window.localStorage.getItem(DENSITY_STORAGE_KEY)).toBe('"compact"')
    expect(document.documentElement.dataset.density).toBe('compact')
  })

  it('shows the organization name as plain text until the switcher exists', () => {
    render(
      <ThemeContext.Provider value={themeValue()}>
        <MemoryRouter initialEntries={['/issues']}>
          <Routes>
            <Route element={<AppLayout org={{ id: 'o1', name: 'Acme', slug: 'acme' }} />}>
              <Route path="/issues" element={<p>Lista</p>} />
            </Route>
          </Routes>
        </MemoryRouter>
      </ThemeContext.Provider>,
    )
    expect(screen.getByText('Acme')).toBeInTheDocument()
  })
})
