import { render, waitFor } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { useAuthStore } from '@/adapters/in/state/auth.store'
import { useOrgStore } from '@/adapters/in/state/org.store'
import { useProjectsStore } from '@/adapters/in/state/projects.store'
import { ProjectHttpRepository } from '@/adapters/out/http/project-http-repository'
import { ThemeContext, type ThemeContextValue } from '@/app/theme-context'
import type { User } from '@/core/auth/domain/user'

import { AppLayout } from './AppLayout'

vi.mock('./NotificationsBell', () => ({
  NotificationsBell: () => <button type="button" aria-label="Notificaciones">campana</button>,
}))

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

const theme: ThemeContextValue = { themeMode: 'light', setThemeMode: () => undefined, toggleTheme: () => undefined }

function renderAt(path: string) {
  return render(
    <ThemeContext.Provider value={theme}>
      <MemoryRouter initialEntries={[path]}>
        <Routes>
          <Route element={<AppLayout />}>
            <Route path="/projects/:id/board" element={<p>Tablero</p>} />
            <Route path="/issues" element={<p>Incidencias</p>} />
          </Route>
        </Routes>
      </MemoryRouter>
    </ThemeContext.Provider>,
  )
}

describe('AppLayout project context', () => {
  beforeEach(() => {
    localStorage.clear()
    useAuthStore.setState({ user, status: 'authenticated', sessionChecked: true })
    useProjectsStore.setState({ projects: [], status: 'idle', error: null })
    useOrgStore.setState({ orgs: [], currentOrgId: 'o1', loaded: true })
    vi.restoreAllMocks()
  })

  it('loads the projects list when the URL points at a project that is not loaded', async () => {
    const list = vi.spyOn(ProjectHttpRepository.prototype, 'list').mockResolvedValue([])
    renderAt('/projects/p1/board')
    await waitFor(() => expect(list).toHaveBeenCalledTimes(1))
  })

  it('does not fetch the projects in the global context', async () => {
    const list = vi.spyOn(ProjectHttpRepository.prototype, 'list').mockResolvedValue([])
    renderAt('/issues')
    await new Promise((resolve) => setTimeout(resolve, 0))
    expect(list).not.toHaveBeenCalled()
  })
})
