import { App } from 'antd'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { login } = vi.hoisted(() => ({ login: vi.fn() }))

vi.mock('@/adapters/in/state/auth.store', () => ({
  useAuthStore: (selector: (state: unknown) => unknown) => selector({ login }),
}))

import { LoginPage } from './LoginPage'

function renderPage() {
  render(
    <App>
      <MemoryRouter>
        <LoginPage />
      </MemoryRouter>
    </App>,
  )
}

describe('LoginPage', () => {
  beforeEach(() => {
    login.mockReset().mockResolvedValue(undefined)
  })

  it('shows the Spanish validation messages when the form is submitted empty', async () => {
    renderPage()
    fireEvent.click(screen.getByRole('button', { name: 'Iniciar sesión' }))
    expect(await screen.findByText('Introduce un correo electrónico válido')).toBeInTheDocument()
    expect(await screen.findByText('La contraseña es obligatoria')).toBeInTheDocument()
    expect(login).not.toHaveBeenCalled()
  })

  it('submits the credentials when they are valid', async () => {
    renderPage()
    fireEvent.change(screen.getByLabelText('Correo electrónico'), { target: { value: 'ana@corelog.test' } })
    fireEvent.change(screen.getByLabelText('Contraseña'), { target: { value: 'Secreta123' } })
    fireEvent.click(screen.getByRole('button', { name: 'Iniciar sesión' }))

    await waitFor(() => expect(login).toHaveBeenCalledWith({ email: 'ana@corelog.test', password: 'Secreta123' }))
  })

  it('offers a link to create an account', () => {
    renderPage()
    expect(screen.getByRole('link', { name: 'Crear una' })).toHaveAttribute('href', '/register')
  })
})
