import { render, screen, within } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { describe, expect, it } from 'vitest'

import { Breadcrumbs } from './Breadcrumbs'

function renderTrail(items: Parameters<typeof Breadcrumbs>[0]['items']) {
  return render(
    <MemoryRouter>
      <Breadcrumbs items={items} />
    </MemoryRouter>,
  )
}

describe('Breadcrumbs', () => {
  it('renders a navigation landmark named "Ruta de navegación" with an ordered list', () => {
    renderTrail([{ label: 'Proyectos', to: '/projects' }, { label: 'CoreLog' }])
    const nav = screen.getByRole('navigation', { name: 'Ruta de navegación' })
    expect(within(nav).getAllByRole('listitem')).toHaveLength(2)
  })

  it('links every item with a route except the last one', () => {
    renderTrail([
      { label: 'Inicio', to: '/' },
      { label: 'Proyectos', to: '/projects' },
      { label: 'CoreLog' },
    ])
    expect(screen.getByRole('link', { name: 'Inicio' })).toHaveAttribute('href', '/')
    expect(screen.getByRole('link', { name: 'Proyectos' })).toHaveAttribute('href', '/projects')
    expect(screen.queryByRole('link', { name: 'CoreLog' })).not.toBeInTheDocument()
  })

  it('marks the last item as the current page', () => {
    renderTrail([{ label: 'Proyectos', to: '/projects' }, { label: 'CoreLog' }])
    expect(screen.getByText('CoreLog')).toHaveAttribute('aria-current', 'page')
    expect(screen.getByRole('link', { name: 'Proyectos' })).not.toHaveAttribute('aria-current')
  })

  it('renders an item without a route as plain text', () => {
    renderTrail([{ label: 'Ajustes' }, { label: 'Seguridad' }])
    expect(screen.getByText('Ajustes')).not.toHaveAttribute('href')
    expect(screen.getByText('Ajustes').tagName).not.toBe('A')
  })

  it('renders nothing for an empty trail', () => {
    renderTrail([])
    expect(screen.queryByRole('navigation')).not.toBeInTheDocument()
  })
})
