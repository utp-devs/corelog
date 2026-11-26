import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { CardGrid } from './CardGrid'

describe('CardGrid', () => {
  it('marks itself as a card grid for the layout audit and renders every card', () => {
    const { container } = render(
      <CardGrid>
        <article>Equipo A</article>
        <article>Equipo B</article>
      </CardGrid>,
    )
    expect(container.firstElementChild).toHaveAttribute('data-card-grid')
    expect(screen.getByText('Equipo A')).toBeInTheDocument()
    expect(screen.getByText('Equipo B')).toBeInTheDocument()
  })

  it('defaults the minimum card width to 280 px and accepts an override', () => {
    const { container, rerender } = render(<CardGrid>x</CardGrid>)
    const grid = container.firstElementChild as HTMLElement
    expect(grid.style.getPropertyValue('--cl-card-min')).toBe('280px')
    rerender(<CardGrid minCardWidth={200}>x</CardGrid>)
    expect(grid.style.getPropertyValue('--cl-card-min')).toBe('200px')
  })

  it('names the grid as a group when ariaLabel is given', () => {
    render(
      <CardGrid ariaLabel="Proyectos">
        <article>P1</article>
      </CardGrid>,
    )
    expect(screen.getByRole('group', { name: 'Proyectos' })).toBeInTheDocument()
  })
})
