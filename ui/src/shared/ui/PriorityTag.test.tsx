import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { PriorityTag } from './PriorityTag'

describe('PriorityTag', () => {
  it('renders the critical priority label', () => {
    render(<PriorityTag priority="critical" />)
    expect(screen.getByText('Crítica')).toBeInTheDocument()
  })

  it('renders the low priority label', () => {
    render(<PriorityTag priority="low" />)
    expect(screen.getByText('Baja')).toBeInTheDocument()
  })

  it('renders the medium and high priority labels', () => {
    const { unmount } = render(<PriorityTag priority="medium" />)
    expect(screen.getByText('Media')).toBeInTheDocument()
    unmount()
    render(<PriorityTag priority="high" />)
    expect(screen.getByText('Alta')).toBeInTheDocument()
  })
})
