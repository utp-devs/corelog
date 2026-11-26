import { fireEvent, render, screen, within } from '@testing-library/react'
import { createMemoryRouter, Link, MemoryRouter, Route, RouterProvider, Routes } from 'react-router-dom'
import { describe, expect, it } from 'vitest'

import { useUnsavedChangesGuard } from './useUnsavedChangesGuard'

function FormScreen({ dirty }: { dirty: boolean }) {
  const guard = useUnsavedChangesGuard(dirty)
  return (
    <>
      {guard}
      <p>Formulario</p>
      <Link to="/destino">Ir al destino</Link>
    </>
  )
}

function renderInDataRouter(dirty: boolean) {
  const router = createMemoryRouter(
    [
      { path: '/formulario', element: <FormScreen dirty={dirty} /> },
      { path: '/destino', element: <p>Destino</p> },
    ],
    { initialEntries: ['/formulario'] },
  )
  render(<RouterProvider router={router} />)
}

describe('useUnsavedChangesGuard', () => {
  it('prevents the browser unload while the form is dirty and not when clean', () => {
    const { unmount } = render(
      <MemoryRouter>
        <FormScreen dirty />
      </MemoryRouter>,
    )
    const dirtyEvent = new Event('beforeunload', { cancelable: true })
    window.dispatchEvent(dirtyEvent)
    expect(dirtyEvent.defaultPrevented).toBe(true)
    unmount()

    render(
      <MemoryRouter>
        <FormScreen dirty={false} />
      </MemoryRouter>,
    )
    const cleanEvent = new Event('beforeunload', { cancelable: true })
    window.dispatchEvent(cleanEvent)
    expect(cleanEvent.defaultPrevented).toBe(false)
  })

  it('blocks in-app navigation with a dialog when the form is dirty, and stays on "Seguir editando"', async () => {
    renderInDataRouter(true)

    fireEvent.click(screen.getByRole('link', { name: 'Ir al destino' }))

    const dialog = await screen.findByRole('dialog')
    expect(within(dialog).getByText('Tienes cambios sin guardar')).toBeInTheDocument()
    fireEvent.click(within(dialog).getByRole('button', { name: 'Seguir editando' }))

    expect(screen.getByText('Formulario')).toBeInTheDocument()
    expect(screen.queryByText('Destino')).not.toBeInTheDocument()
  })

  it('navigates after the user confirms with "Salir"', async () => {
    renderInDataRouter(true)

    fireEvent.click(screen.getByRole('link', { name: 'Ir al destino' }))
    const dialog = await screen.findByRole('dialog')
    fireEvent.click(within(dialog).getByRole('button', { name: 'Salir' }))

    expect(await screen.findByText('Destino')).toBeInTheDocument()
  })

  it('lets navigation through without a dialog when the form is clean', () => {
    renderInDataRouter(false)

    fireEvent.click(screen.getByRole('link', { name: 'Ir al destino' }))

    expect(screen.getByText('Destino')).toBeInTheDocument()
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('works outside a data router: no dialog and no crash, only the unload warning applies', () => {
    render(
      <MemoryRouter initialEntries={['/formulario']}>
        <Routes>
          <Route path="/formulario" element={<FormScreen dirty />} />
          <Route path="/destino" element={<p>Destino</p>} />
        </Routes>
      </MemoryRouter>,
    )
    fireEvent.click(screen.getByRole('link', { name: 'Ir al destino' }))
    expect(screen.getByText('Destino')).toBeInTheDocument()
  })
})
