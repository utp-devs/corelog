import { App } from 'antd'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import type { ReactElement } from 'react'
import { MemoryRouter } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { AppError } from '@/core/shared/app-error'

const { login, register } = vi.hoisted(() => ({ login: vi.fn(), register: vi.fn() }))

vi.mock('@/adapters/in/state/auth.store', () => ({
  useAuthStore: (selector: (state: unknown) => unknown) => selector({ login, register }),
}))

import { LoginPage } from './LoginPage'
import { RegisterPage } from './RegisterPage'

function renderIn(ui: ReactElement) {
  return render(
    <App>
      <MemoryRouter>{ui}</MemoryRouter>
    </App>,
  )
}

describe('auth screens: landmarks and labels', () => {
  beforeEach(() => {
    login.mockReset().mockResolvedValue(undefined)
    register.mockReset().mockResolvedValue(undefined)
  })

  it('exposes a skip link to the main landmark and a level-1 heading, with no brand aside', () => {
    const { container } = renderIn(<LoginPage />)
    expect(screen.getByRole('link', { name: 'Saltar al formulario' })).toHaveAttribute('href', '#auth-main')
    expect(container.querySelector('main#auth-main')).not.toBeNull()
    expect(screen.getByRole('heading', { level: 1, name: 'Iniciar sesión' })).toBeInTheDocument()
    expect(screen.queryByRole('complementary')).not.toBeInTheDocument()
  })

  it('renders the brand banner inside the single centered card', () => {
    const { container } = renderIn(<LoginPage />)
    const card = container.querySelector('main#auth-main section')
    expect(card).not.toBeNull()
    expect(card?.querySelector('img[alt="CoreLog"]')).not.toBeNull()
  })

  it('associates each input with its visible label and marks it required', () => {
    renderIn(<LoginPage />)
    const email = screen.getByLabelText('Correo electrónico')
    expect(email).toHaveAttribute('aria-required', 'true')
    expect(email).toHaveAttribute('autocomplete', 'email')
    expect(email).toHaveAttribute('type', 'email')
  })
})

describe('LoginPage form behaviour', () => {
  beforeEach(() => {
    login.mockReset().mockResolvedValue(undefined)
  })

  it('uses current-password autocomplete on the password field', () => {
    renderIn(<LoginPage />)
    expect(screen.getByLabelText('Contraseña')).toHaveAttribute('autocomplete', 'current-password')
    expect(screen.getByLabelText('Contraseña')).toHaveAttribute('type', 'password')
  })

  it('toggles password visibility from a keyboard-reachable button', () => {
    renderIn(<LoginPage />)
    const toggle = screen.getByRole('button', { name: 'Mostrar contraseña' })
    expect(toggle).toHaveAttribute('aria-pressed', 'false')
    fireEvent.click(toggle)
    expect(screen.getByLabelText('Contraseña')).toHaveAttribute('type', 'text')
    expect(toggle).toHaveAttribute('aria-pressed', 'true')
    fireEvent.click(toggle)
    expect(screen.getByLabelText('Contraseña')).toHaveAttribute('type', 'password')
  })

  it('shows a generic alert for a failure without field details', async () => {
    login.mockRejectedValue(new AppError({ status: 401, code: 'auth.invalid_credentials', message: 'invalid email or password' }))
    renderIn(<LoginPage />)
    fireEvent.change(screen.getByLabelText('Correo electrónico'), { target: { value: 'ana@corelog.test' } })
    fireEvent.change(screen.getByLabelText('Contraseña'), { target: { value: 'Secreta123' } })
    fireEvent.click(screen.getByRole('button', { name: 'Iniciar sesión' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('invalid email or password')
  })

  it('maps server details to the email field and marks it invalid', async () => {
    login.mockRejectedValue(
      new AppError({
        status: 422,
        code: 'validation.failed',
        message: 'invalid request body',
        details: [{ field: 'email', message: 'debe ser un correo válido' }],
      }),
    )
    renderIn(<LoginPage />)
    fireEvent.change(screen.getByLabelText('Correo electrónico'), { target: { value: 'ana@corelog.test' } })
    fireEvent.change(screen.getByLabelText('Contraseña'), { target: { value: 'Secreta123' } })
    fireEvent.click(screen.getByRole('button', { name: 'Iniciar sesión' }))
    const email = await screen.findByLabelText('Correo electrónico')
    await waitFor(() => expect(email).toHaveAttribute('aria-invalid', 'true'))
    expect(email).toHaveAccessibleDescription('debe ser un correo válido')
  })
})

describe('RegisterPage form behaviour', () => {
  beforeEach(() => {
    register.mockReset().mockResolvedValue(undefined)
  })

  it('uses the autocomplete tokens for name, email and new password', () => {
    renderIn(<RegisterPage />)
    expect(screen.getByLabelText('Nombre')).toHaveAttribute('autocomplete', 'name')
    expect(screen.getByLabelText('Correo electrónico')).toHaveAttribute('autocomplete', 'email')
    expect(screen.getByLabelText('Contraseña')).toHaveAttribute('autocomplete', 'new-password')
  })

  it('announces the password strength as the user types', () => {
    renderIn(<RegisterPage />)
    fireEvent.change(screen.getByLabelText('Contraseña'), { target: { value: 'Secreta123!456' } })
    expect(screen.getByText('Seguridad de la contraseña: Fuerte')).toBeInTheDocument()
  })

  it('shows the terms line', () => {
    renderIn(<RegisterPage />)
    expect(screen.getByText(/condiciones de uso y la política de privacidad/)).toBeInTheDocument()
  })
})
