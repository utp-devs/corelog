import { fireEvent, render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { describe, expect, it, vi } from 'vitest'

import { ErrorPage } from './ErrorPage'

function renderPage(props: Parameters<typeof ErrorPage>[0]) {
  return render(
    <MemoryRouter>
      <ErrorPage {...props} />
    </MemoryRouter>,
  )
}

describe('ErrorPage', () => {
  it('shows the Spanish 403 copy and the status code', () => {
    renderPage({ variant: 403 })
    expect(screen.getByRole('heading', { level: 1, name: 'No tienes acceso a esta página' })).toBeInTheDocument()
    expect(screen.getByText('403')).toBeInTheDocument()
  })

  it('shows the Spanish 404 copy', () => {
    renderPage({ variant: 404 })
    expect(screen.getByRole('heading', { name: 'No encontramos esta página' })).toBeInTheDocument()
    expect(screen.getByText('404')).toBeInTheDocument()
  })

  it('shows the Spanish 500 copy', () => {
    renderPage({ variant: 500 })
    expect(screen.getByRole('heading', { name: 'Algo salió mal' })).toBeInTheDocument()
    expect(screen.getByText('500')).toBeInTheDocument()
  })

  it('always offers "Volver al inicio" as a link to /', () => {
    renderPage({ variant: 404 })
    expect(screen.getByRole('link', { name: 'Volver al inicio' })).toHaveAttribute('href', '/')
  })

  it('offers a retry action on 500 when onRetry is given, and calls it', () => {
    const onRetry = vi.fn()
    renderPage({ variant: 500, onRetry })
    fireEvent.click(screen.getByRole('button', { name: 'Reintentar' }))
    expect(onRetry).toHaveBeenCalledTimes(1)
  })

  it('does not offer retry on 403 or 404 even when onRetry is given', () => {
    renderPage({ variant: 403, onRetry: () => {} })
    expect(screen.queryByRole('button', { name: 'Reintentar' })).not.toBeInTheDocument()
  })

  it('does not offer retry on 500 without onRetry', () => {
    renderPage({ variant: 500 })
    expect(screen.queryByRole('button', { name: 'Reintentar' })).not.toBeInTheDocument()
  })

  it('accepts custom title and description', () => {
    renderPage({ variant: 404, title: 'Incidencia no encontrada', description: 'Puede que la hayan eliminado.' })
    expect(screen.getByRole('heading', { name: 'Incidencia no encontrada' })).toBeInTheDocument()
    expect(screen.getByText('Puede que la hayan eliminado.')).toBeInTheDocument()
  })
})
