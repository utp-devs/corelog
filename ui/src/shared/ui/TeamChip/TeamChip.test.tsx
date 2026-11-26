import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { TeamChip } from './TeamChip'

describe('TeamChip', () => {
  it('shows the name with the plural member count', () => {
    render(<TeamChip id="t-1" name="Plataforma" memberCount={4} />)
    expect(screen.getByText('Plataforma')).toBeInTheDocument()
    expect(screen.getByText('4 miembros')).toBeInTheDocument()
  })

  it('uses the singular for one member and hides the count when absent', () => {
    const { unmount } = render(<TeamChip id="t-2" name="Soporte" memberCount={1} />)
    expect(screen.getByText('1 miembro')).toBeInTheDocument()
    unmount()
    render(<TeamChip id="t-3" name="Soporte" />)
    expect(screen.queryByText(/miembro/)).not.toBeInTheDocument()
  })

  it('renders a link only when href is given', () => {
    render(<TeamChip id="t-4" name="Diseño" href="/equipos/t-4" />)
    expect(screen.getByRole('link', { name: /Diseño/ })).toHaveAttribute('href', '/equipos/t-4')
  })

  it('renders a hostile name as text', () => {
    const { container } = render(<TeamChip id="t-5" name="<img src=x onerror=alert(1)>" />)
    expect(container.querySelector('img')).toBeNull()
  })
})

describe('TeamChip text wrapping', () => {
  it('exposes the full team name and member count as title so truncated text keeps a tooltip', () => {
    render(<TeamChip id="t-1" name="Plataforma de datos internos" memberCount={3} />)
    expect(screen.getByText('Plataforma de datos internos')).toHaveAttribute('title', 'Plataforma de datos internos')
    expect(screen.getByText('3 miembros')).toHaveAttribute('title', '3 miembros')
  })
})
