import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { describe, expect, it } from 'vitest'

import { NotFoundPage } from './NotFoundPage'

describe('NotFoundPage', () => {
  it('renders 404 message and return button', () => {
    render(
      <MemoryRouter>
        <NotFoundPage />
      </MemoryRouter>
    )

    expect(screen.getByText('404')).toBeInTheDocument()
    expect(screen.getByText('Lo sentimos, la página que buscas no existe.')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Volver a incidencias/i })).toBeInTheDocument()
  })
})
