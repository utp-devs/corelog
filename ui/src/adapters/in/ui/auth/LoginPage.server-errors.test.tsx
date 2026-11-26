import { App } from 'antd'
import { fireEvent, render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { AppError } from '@/core/shared/app-error'

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

function fillAndSubmit() {
  fireEvent.change(screen.getByLabelText('Correo electrónico'), { target: { value: 'ana@corelog.test' } })
  fireEvent.change(screen.getByLabelText('Contraseña'), { target: { value: 'Secreta123' } })
  fireEvent.click(screen.getByRole('button', { name: 'Iniciar sesión' }))
}

describe('LoginPage server field errors', () => {
  beforeEach(() => {
    login.mockReset()
  })

  it('shows the server detail under the field it refers to', async () => {
    login.mockRejectedValue(
      new AppError({
        status: 422,
        code: 'validation.failed',
        message: 'invalid request body',
        serverMessage: 'invalid request body',
        details: [{ field: 'email', message: 'debe ser un correo válido' }],
      }),
    )
    renderPage()
    fillAndSubmit()

    expect(await screen.findByText('debe ser un correo válido')).toBeInTheDocument()
  })

  it('shows the error message in a toast when there are no field details', async () => {
    login.mockRejectedValue(new AppError({ status: 401, code: 'auth.invalid_credentials', message: 'invalid email or password' }))
    renderPage()
    fillAndSubmit()

    expect(await screen.findByText('invalid email or password')).toBeInTheDocument()
  })
})
