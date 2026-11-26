import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { ProjectAvatar } from './ProjectAvatar'

describe('ProjectAvatar', () => {
  it('shows the uppercase key initials and names the project', () => {
    render(<ProjectAvatar name="Corelog" projectKey="cl" />)
    const tile = screen.getByRole('img', { name: 'Proyecto Corelog' })
    expect(tile).toHaveTextContent('CL')
  })

  it('shows the icon instead of initials when one is given', () => {
    render(<ProjectAvatar name="Corelog" projectKey="CL" icon={<span>icono</span>} />)
    expect(screen.getByText('icono')).toBeInTheDocument()
    expect(screen.getByRole('img', { name: 'Proyecto Corelog' })).not.toHaveTextContent('CL')
  })

  it('picks the same palette slot for the same key', () => {
    const first = render(<ProjectAvatar name="A" projectKey="OPS" />)
    const second = render(<ProjectAvatar name="B" projectKey="OPS" />)
    const a = first.container.firstElementChild?.getAttribute('data-palette')
    const b = second.container.firstElementChild?.getAttribute('data-palette')
    expect(a).toBeTruthy()
    expect(a).toBe(b)
  })

  it('renders a hostile name as text', () => {
    const { container } = render(<ProjectAvatar name="<script>x</script>" projectKey="XS" />)
    expect(container.querySelector('script')).toBeNull()
  })
})
