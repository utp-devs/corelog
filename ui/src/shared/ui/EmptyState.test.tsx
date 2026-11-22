import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { EmptyState } from './EmptyState'

describe('EmptyState', () => {
  it('renders the title and the optional description', () => {
    render(<EmptyState title="Aún no hay incidencias" description="Crea la primera para empezar" />)
    expect(screen.getByText('Aún no hay incidencias')).toBeInTheDocument()
    expect(screen.getByText('Crea la primera para empezar')).toBeInTheDocument()
  })

  it('renders the action when provided', () => {
    render(<EmptyState title="Vacío" action={<button type="button">Crear</button>} />)
    expect(screen.getByRole('button', { name: 'Crear' })).toBeInTheDocument()
  })

  it('omits the description and action when not provided', () => {
    render(<EmptyState title="Vacío" />)
    expect(screen.getByText('Vacío')).toBeInTheDocument()
    expect(screen.queryByRole('button')).not.toBeInTheDocument()
  })
})
