import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { StatTile } from './StatTile'

describe('StatTile', () => {
  it('renders the label and the value', () => {
    render(<StatTile label="Abiertas" value={12} />)
    expect(screen.getByText('Abiertas')).toBeInTheDocument()
    expect(screen.getByText('12')).toBeInTheDocument()
  })

  it('renders the hint only when provided', () => {
    const { unmount } = render(<StatTile label="Vencidas" value={3} hint="+1 esta semana" />)
    expect(screen.getByText('+1 esta semana')).toBeInTheDocument()
    unmount()
    render(<StatTile label="Vencidas" value={3} />)
    expect(screen.queryByText('+1 esta semana')).not.toBeInTheDocument()
  })
})
