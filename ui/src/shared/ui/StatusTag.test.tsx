import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { StatusTag } from './StatusTag'

describe('StatusTag', () => {
  it('renders the open status label', () => {
    render(<StatusTag status="open" />)
    expect(screen.getByText('Abierto')).toBeInTheDocument()
  })

  it('renders the in_progress status label', () => {
    render(<StatusTag status="in_progress" />)
    expect(screen.getByText('En progreso')).toBeInTheDocument()
  })

  it('renders the resolved and closed status labels', () => {
    const { unmount } = render(<StatusTag status="resolved" />)
    expect(screen.getByText('Resuelto')).toBeInTheDocument()
    unmount()
    render(<StatusTag status="closed" />)
    expect(screen.getByText('Cerrado')).toBeInTheDocument()
  })
})
