import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { LoadingSkeleton } from './LoadingSkeleton'

describe('LoadingSkeleton', () => {
  it('announces the loading state to assistive technology', () => {
    render(<LoadingSkeleton variant="table" />)
    const status = screen.getByRole('status', { name: 'Cargando' })
    expect(status).toHaveAttribute('aria-busy', 'true')
  })

  it('renders every variant without errors', () => {
    const { unmount: unmountTable } = render(<LoadingSkeleton variant="table" rows={3} />)
    unmountTable()
    const { unmount: unmountCard } = render(<LoadingSkeleton variant="card" />)
    unmountCard()
    render(<LoadingSkeleton variant="detail" />)
    expect(screen.getByRole('status', { name: 'Cargando' })).toBeInTheDocument()
  })
})
