import { render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { SplitLayout } from './SplitLayout'

// Mocks matchMedia: the stacked query "(max-width: 991.98px)" matches when `narrow` is true.
function mockViewport(narrow: boolean) {
  const original = window.matchMedia
  window.matchMedia = vi.fn((query: string) => ({
    matches: narrow && query === '(max-width: 991.98px)',
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

describe('SplitLayout', () => {
  let restore: () => void = () => {}

  beforeEach(() => {
    restore = mockViewport(false)
  })

  afterEach(() => {
    restore()
  })

  it('renders main content and the side panel in a named aside landmark', () => {
    render(<SplitLayout main={<p>Incidencia ABC-1</p>} side={<p>Detalles de la incidencia</p>} />)
    expect(screen.getByText('Incidencia ABC-1')).toBeInTheDocument()
    expect(screen.getByRole('complementary', { name: 'Panel lateral' })).toHaveTextContent('Detalles de la incidencia')
  })

  it('uses sideLabel as the landmark name', () => {
    render(<SplitLayout main="m" side="s" sideLabel="Detalles" />)
    expect(screen.getByRole('complementary', { name: 'Detalles' })).toBeInTheDocument()
  })

  it('defaults to a 320 px side panel and accepts 360 and 400', () => {
    const { container, rerender } = render(<SplitLayout main="m" side="s" />)
    const layout = container.firstElementChild as HTMLElement
    expect(layout).toHaveAttribute('data-side-width', '320')

    rerender(<SplitLayout main="m" side="s" sideWidth={400} />)
    expect(container.firstElementChild).toHaveAttribute('data-side-width', '400')
  })

  it('is not stacked on wide viewports', () => {
    const { container } = render(<SplitLayout main="m" side="s" />)
    expect(container.firstElementChild).toHaveAttribute('data-stacked', 'false')
  })

  it('stacks the side panel below the main content under 992 px', () => {
    restore()
    restore = mockViewport(true)
    const { container } = render(<SplitLayout main="m" side="s" />)
    expect(container.firstElementChild).toHaveAttribute('data-stacked', 'true')
  })
})
