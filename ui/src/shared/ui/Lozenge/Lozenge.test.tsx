import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { Lozenge } from './Lozenge'

describe('Lozenge', () => {
  it('always shows the text', () => {
    render(<Lozenge category="done">Hecho</Lozenge>)
    expect(screen.getByText('Hecho')).toBeVisible()
  })

  it('uses the category when given and the tone otherwise', () => {
    const { unmount } = render(<Lozenge category="progress" tone="danger">En curso</Lozenge>)
    expect(screen.getByText('En curso')).toHaveAttribute('data-variant', 'progress')
    unmount()
    render(<Lozenge tone="warning">Revisar</Lozenge>)
    expect(screen.getByText('Revisar')).toHaveAttribute('data-variant', 'warning')
  })

  it('defaults to the neutral tone', () => {
    render(<Lozenge>Sin estado</Lozenge>)
    expect(screen.getByText('Sin estado')).toHaveAttribute('data-variant', 'neutral')
  })

  it('renders hostile text as text', () => {
    const { container } = render(<Lozenge tone="info">{'<img src=x onerror=alert(1)>'}</Lozenge>)
    expect(container.querySelector('img')).toBeNull()
  })
})
