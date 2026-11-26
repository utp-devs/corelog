import { App } from 'antd'
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { createMemoryRouter, RouterProvider } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { submitMock } = vi.hoisted(() => ({ submitMock: vi.fn() }))

vi.mock('./hooks/useCreateTicket', () => ({
  useCreateTicket: () => ({ submit: submitMock, isSubmitting: false }),
}))

import { NewTicketPage } from './NewTicketPage'

function renderPage() {
  const router = createMemoryRouter(
    [
      { path: '/issues/new', element: <NewTicketPage /> },
      { path: '/issues', element: <p>Listado de incidencias</p> },
      { path: '/issues/:id', element: <p>Detalle de la incidencia</p> },
    ],
    { initialEntries: ['/issues/new'] },
  )
  render(
    <App>
      <RouterProvider router={router} />
    </App>,
  )
  return router
}

function fillTitle(value: string) {
  fireEvent.change(screen.getByLabelText(/Título/), { target: { value } })
}

describe('NewTicketPage', () => {
  beforeEach(() => {
    submitMock.mockReset()
  })

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
    expect(submitMock).not.toHaveBeenCalled()
  })

  it('leaves the page without a prompt when nothing was typed', async () => {
    const router = renderPage()
    fireEvent.click(screen.getByRole('button', { name: 'Cancelar' }))
    await waitFor(() => expect(router.state.location.pathname).toBe('/issues'))
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('asks before leaving with typed values and stays when the user keeps editing', async () => {
    const router = renderPage()
    fillTitle('Pantalla en blanco')

    fireEvent.click(screen.getByRole('button', { name: 'Cancelar' }))
    const dialog = await screen.findByRole('dialog')
    expect(within(dialog).getByText('Tienes cambios sin guardar')).toBeInTheDocument()
    fireEvent.click(within(dialog).getByRole('button', { name: 'Seguir editando' }))

    expect(router.state.location.pathname).toBe('/issues/new')
    expect(screen.getByLabelText(/Título/)).toHaveValue('Pantalla en blanco')
  })

  it('leaves after confirming the discard dialog', async () => {
    const router = renderPage()
    fillTitle('Pantalla en blanco')

    fireEvent.click(screen.getByRole('button', { name: 'Cancelar' }))
    const dialog = await screen.findByRole('dialog')
    fireEvent.click(within(dialog).getByRole('button', { name: 'Salir' }))

    await waitFor(() => expect(router.state.location.pathname).toBe('/issues'))
  })

  it('redirects to the new issue after a successful create without showing the prompt', async () => {
    submitMock.mockResolvedValue({ id: 'issue-9' })
    const router = renderPage()
    fillTitle('Pantalla en blanco')
    fireEvent.change(screen.getByRole('textbox', { name: 'Descripción' }), { target: { value: 'Al abrir el panel' } })

    fireEvent.click(screen.getByRole('button', { name: 'Crear incidencia' }))

    await waitFor(() => expect(router.state.location.pathname).toBe('/issues/issue-9'))
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(submitMock).toHaveBeenCalledWith(expect.objectContaining({ title: 'Pantalla en blanco' }))
  })

  it('stays on the form with the typed values when the server refuses the issue', async () => {
    submitMock.mockRejectedValue(new Error('El proyecto está archivado'))
    const router = renderPage()
    fillTitle('Pantalla en blanco')
    fireEvent.change(screen.getByRole('textbox', { name: 'Descripción' }), { target: { value: 'Al abrir el panel' } })

    fireEvent.click(screen.getByRole('button', { name: 'Crear incidencia' }))

    expect(await screen.findByText('El proyecto está archivado')).toBeInTheDocument()
    expect(router.state.location.pathname).toBe('/issues/new')
    expect(screen.getByLabelText(/Título/)).toHaveValue('Pantalla en blanco')
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
