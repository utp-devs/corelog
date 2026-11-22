import { App } from 'antd'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { register } = vi.hoisted(() => ({ register: vi.fn() }))

vi.mock('@/adapters/in/state/auth.store', () => ({
  useAuthStore: (selector: (state: unknown) => unknown) => selector({ register }),
}))

import { RegisterPage } from './RegisterPage'

function renderPage() {
  render(
    <App>
      <MemoryRouter>
        <RegisterPage />
      </MemoryRouter>
    </App>,
  )
}

describe('RegisterPage', () => {
  beforeEach(() => {
    register.mockReset().mockResolvedValue(undefined)
  })

  it('explains the minimum length of the name and password in Spanish', async () => {
    renderPage()
    fireEvent.change(screen.getByLabelText('Nombre'), { target: { value: 'A' } })
    fireEvent.change(screen.getByLabelText('Contraseña'), { target: { value: 'corta' } })
    fireEvent.click(screen.getByRole('button', { name: 'Crear cuenta' }))

    expect(await screen.findByText('El nombre debe tener al menos 2 caracteres')).toBeInTheDocument()
    expect(await screen.findByText('La contraseña debe tener al menos 8 caracteres')).toBeInTheDocument()
    expect(register).not.toHaveBeenCalled()
  })

  it('creates the account when every field is valid', async () => {
    renderPage()
    fireEvent.change(screen.getByLabelText('Nombre'), { target: { value: 'Ana Torres' } })
    fireEvent.change(screen.getByLabelText('Correo electrónico'), { target: { value: 'ana@corelog.test' } })
    fireEvent.change(screen.getByLabelText('Contraseña'), { target: { value: 'Secreta123' } })
    fireEvent.click(screen.getByRole('button', { name: 'Crear cuenta' }))

    await waitFor(() => expect(register).toHaveBeenCalledWith({
      name: 'Ana Torres',
      email: 'ana@corelog.test',
      password: 'Secreta123',
    }))
  })
})
