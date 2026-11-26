import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { Toolbar } from './Toolbar'

// Mocks matchMedia: the overflow query "(max-width: 1279.98px)" matches when `narrow` is true.
function mockViewport(narrow: boolean) {
  const original = window.matchMedia
  window.matchMedia = vi.fn((query: string) => ({
    matches: narrow && query === '(max-width: 1279.98px)',
    media: query,
    onchange: null,
    addListener: () => {},
    removeListener: () => {},
    addEventListener: () => {},
    removeEventListener: () => {},
    dispatchEvent: () => false,
  })) as unknown as typeof window.matchMedia
  return () => {
    window.matchMedia = original
  }
}

describe('Toolbar', () => {
  let restore: () => void = () => {}

  beforeEach(() => {
    restore = mockViewport(false)
  })

  afterEach(() => {
    restore()
  })

  it('is exposed as a named toolbar and renders left and right slots', () => {
    render(<Toolbar left={<input aria-label="Buscar" />} right={<button type="button">Crear</button>} />)
    expect(screen.getByRole('toolbar', { name: 'Barra de acciones' })).toBeInTheDocument()
    expect(screen.getByRole('textbox', { name: 'Buscar' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Crear' })).toBeInTheDocument()
  })

  it('shows secondary actions inline at 1280 px and wider, with no "Más" menu', () => {
    render(<Toolbar right={<button type="button">Crear</button>} secondary={<button type="button">Exportar</button>} />)
    expect(screen.getByRole('button', { name: 'Exportar' })).toBeVisible()
    expect(screen.queryByRole('button', { name: 'Más' })).not.toBeInTheDocument()
  })

  it('moves secondary actions into a "Más" menu below 1280 px', async () => {
    restore()
    restore = mockViewport(true)
    render(<Toolbar right={<button type="button">Crear</button>} secondary={<button type="button">Exportar</button>} />)

    expect(screen.queryByRole('button', { name: 'Exportar' })).not.toBeInTheDocument()
    const more = screen.getByRole('button', { name: 'Más' })
    expect(more).toBeInTheDocument()

    fireEvent.click(more)
    expect(await screen.findByRole('button', { name: 'Exportar' })).toBeInTheDocument()
  })

  it('keeps primary actions visible when the overflow menu is used', async () => {
    restore()
    restore = mockViewport(true)
    render(<Toolbar right={<button type="button">Crear</button>} secondary={<button type="button">Exportar</button>} />)
    fireEvent.click(screen.getByRole('button', { name: 'Más' }))
    await waitFor(() => expect(screen.getByRole('button', { name: 'Exportar' })).toBeInTheDocument())
    expect(screen.getByRole('button', { name: 'Crear' })).toBeInTheDocument()
  })

  it('does not render a "Más" button when there are no secondary actions', () => {
    restore()
    restore = mockViewport(true)
    render(<Toolbar right={<button type="button">Crear</button>} />)
    expect(screen.queryByRole('button', { name: 'Más' })).not.toBeInTheDocument()
  })

  it('marks the toolbar as sticky only when requested', () => {
    const { rerender } = render(<Toolbar left="filtros" />)
    expect(screen.getByRole('toolbar')).toHaveAttribute('data-sticky', 'false')
    rerender(<Toolbar left="filtros" sticky />)
    expect(screen.getByRole('toolbar')).toHaveAttribute('data-sticky', 'true')
  })
})
