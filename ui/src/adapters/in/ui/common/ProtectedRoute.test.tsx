import { render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { beforeEach, describe, expect, it } from 'vitest'

import { useAuthStore } from '@/adapters/in/state/auth.store'

import { ProtectedRoute } from './ProtectedRoute'

function renderRoutes() {
  return render(
    <MemoryRouter initialEntries={['/private']}>
      <Routes>
        <Route path="/login" element={<p>Pantalla de acceso</p>} />
        <Route element={<ProtectedRoute />}>
          <Route path="/private" element={<p>Contenido privado</p>} />
        </Route>
      </Routes>
    </MemoryRouter>,
  )
}

describe('ProtectedRoute', () => {
  beforeEach(() => {
    useAuthStore.setState({ user: null, status: 'idle', sessionChecked: false })
  })

  it('shows a labelled loading status while the session is being checked', () => {
    renderRoutes()
    expect(screen.getByRole('status')).toHaveTextContent('Comprobando tu sesión...')
    expect(screen.queryByText('Contenido privado')).not.toBeInTheDocument()
  })

  it('redirects to login when there is no authenticated session', () => {
    useAuthStore.setState({ sessionChecked: true, status: 'idle' })
    renderRoutes()
    expect(screen.getByText('Pantalla de acceso')).toBeInTheDocument()
  })

  it('renders the protected content for an authenticated session', () => {
    useAuthStore.setState({ sessionChecked: true, status: 'authenticated' })
    renderRoutes()
    expect(screen.getByText('Contenido privado')).toBeInTheDocument()
  })
})
