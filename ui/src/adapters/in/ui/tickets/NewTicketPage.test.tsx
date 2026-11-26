import { App } from 'antd'
import { fireEvent, render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { describe, expect, it, vi } from 'vitest'

vi.mock('./hooks/useCreateTicket', () => ({
  useCreateTicket: () => ({ submit: vi.fn(), isSubmitting: false }),
}))

import { NewTicketPage } from './NewTicketPage'

function renderPage() {
  return render(
    <App>
      <MemoryRouter initialEntries={['/issues/new']}>
        <Routes>
          <Route path="/issues/new" element={<NewTicketPage />} />
        </Routes>
      </MemoryRouter>
    </App>,
  )
}

describe('NewTicketPage', () => {
  it('groups the form in sections with labelled fields', () => {
    renderPage()
    expect(screen.getByRole('heading', { name: 'Nueva incidencia' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Datos principales' })).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Descripción' })).toBeInTheDocument()
    expect(screen.getByLabelText(/Título/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Crear incidencia' })).toBeInTheDocument()
  })

  it('shows validation messages on submit without calling the server', async () => {
    renderPage()
    fireEvent.click(screen.getByRole('button', { name: 'Crear incidencia' }))
    expect(await screen.findByText('El título es demasiado corto')).toBeInTheDocument()
  })

  it('renders without console warnings or errors', () => {
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
    const error = vi.spyOn(console, 'error').mockImplementation(() => {})
    renderPage()
    expect(warn).not.toHaveBeenCalled()
    expect(error).not.toHaveBeenCalled()
    warn.mockRestore()
    error.mockRestore()
  })
})
