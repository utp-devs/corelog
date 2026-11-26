import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { UserChip } from './UserChip'

const user = { id: 'u-1', name: 'Ana Pérez', role: 'Administradora' }

describe('UserChip', () => {
  it('shows the name and the optional role', () => {
    render(<UserChip user={user} />)
    expect(screen.getByText('Ana Pérez')).toBeInTheDocument()
    expect(screen.getByText('Administradora')).toBeInTheDocument()
  })

  it('renders a link only when href is given', () => {
    const { unmount } = render(<UserChip user={user} href="/personas/u-1" />)
    expect(screen.getByRole('link', { name: /Ana Pérez/ })).toHaveAttribute('href', '/personas/u-1')
    unmount()
    render(<UserChip user={user} />)
    expect(screen.queryByRole('link')).not.toBeInTheDocument()
  })

  it('renders a hostile name as text', () => {
    const { container } = render(<UserChip user={{ id: 'u-2', name: '<script>alert(1)</script>' }} />)
    expect(container.querySelector('script')).toBeNull()
    expect(screen.getByText('<script>alert(1)</script>')).toBeInTheDocument()
  })
})
