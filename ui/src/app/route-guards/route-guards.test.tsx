import { render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { beforeEach, describe, expect, it } from 'vitest'

import { useAuthStore } from '@/adapters/in/state/auth.store'
import type { User } from '@/core/auth/domain/user'

import { RequireAuth, RequirePlatformAdmin } from './index'

const baseUser: User = {
  id: 'u1',
  name: 'Ana Pérez',
  email: 'ana@example.com',
  role: 'agent',
  teamId: null,
  createdAt: '2026-01-01T00:00:00Z',
}

function setSession(patch: Partial<ReturnType<typeof useAuthStore.getState>>) {
  useAuthStore.setState({ user: null, status: 'idle', sessionChecked: true, ...patch })
}

function renderAt(path: string, element: 'auth' | 'admin') {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/login" element={<p>Pantalla de acceso</p>} />
        <Route element={element === 'auth' ? <RequireAuth /> : <RequirePlatformAdmin />}>
          <Route path="/teams" element={<p>Contenido protegido</p>} />
        </Route>
      </Routes>
    </MemoryRouter>,
  )
}

describe('RequirePlatformAdmin', () => {
  beforeEach(() => {
    setSession({})
  })

  it('renders the 403 page instead of redirecting a non-admin', () => {
    setSession({ user: baseUser, status: 'authenticated' })
    renderAt('/teams', 'admin')
    expect(screen.getByRole('heading', { name: 'No tienes acceso a esta página' })).toBeInTheDocument()
    expect(screen.getByText('403')).toBeInTheDocument()
    expect(screen.queryByText('Contenido protegido')).not.toBeInTheDocument()
  })

  it('renders the nested route for an admin', () => {
    setSession({ user: { ...baseUser, role: 'admin' }, status: 'authenticated' })
    renderAt('/teams', 'admin')
    expect(screen.getByText('Contenido protegido')).toBeInTheDocument()
  })
})

describe('RequireAuth', () => {
  it('sends an anonymous visitor to /login', () => {
    setSession({ status: 'idle' })
    renderAt('/teams', 'auth')
    expect(screen.getByText('Pantalla de acceso')).toBeInTheDocument()
  })

  it('renders nothing while the session is not checked yet', () => {
    setSession({ sessionChecked: false, status: 'loading' })
    const { container } = renderAt('/teams', 'auth')
    expect(container).toBeEmptyDOMElement()
  })

  it('renders the nested route when authenticated', () => {
    setSession({ user: baseUser, status: 'authenticated' })
    renderAt('/teams', 'auth')
    expect(screen.getByText('Contenido protegido')).toBeInTheDocument()
  })
})
