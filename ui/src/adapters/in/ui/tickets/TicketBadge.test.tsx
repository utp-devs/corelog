import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { TicketBadge } from './TicketBadge'

describe('TicketBadge', () => {
  it('renders status open badge correctly', () => {
    render(<TicketBadge kind="status" value="open" />)
    expect(screen.getByText('Abierto')).toBeInTheDocument()
  })

  it('renders status in_progress badge correctly', () => {
    render(<TicketBadge kind="status" value="in_progress" />)
    expect(screen.getByText('En progreso')).toBeInTheDocument()
  })

  it('renders priority critical badge correctly', () => {
    render(<TicketBadge kind="priority" value="critical" />)
    expect(screen.getByText('Critica')).toBeInTheDocument()
  })

  it('renders priority low badge correctly', () => {
    render(<TicketBadge kind="priority" value="low" />)
    expect(screen.getByText('Baja')).toBeInTheDocument()
  })
})
