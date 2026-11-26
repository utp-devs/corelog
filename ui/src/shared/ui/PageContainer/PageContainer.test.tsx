import { render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { PageContainer } from './PageContainer'

// Mocks matchMedia so that the stacked-layout query "(max-width: 991.98px)" matches when `narrow` is true.
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

describe('PageContainer', () => {
  let restore: () => void = () => {}

  beforeEach(() => {
    restore = mockViewport(false)
  })

  afterEach(() => {
    restore()
  })

  it('renders the children inside the container', () => {
    render(
      <PageContainer variant="wide">
        <p>Contenido de la página</p>
      </PageContainer>,
    )
    expect(screen.getByText('Contenido de la página')).toBeInTheDocument()
  })

  it.each(['reading', 'wide', 'fluid'] as const)('exposes the %s variant as data-variant', (variant) => {
    const { container } = render(<PageContainer variant={variant}>x</PageContainer>)
    expect(container.firstElementChild).toHaveAttribute('data-variant', variant)
  })

  it('sets data-ready to "true" when ready is true and "false" otherwise', () => {
    const { container, rerender } = render(
      <PageContainer variant="wide" ready>
        x
      </PageContainer>,
    )
    expect(container.firstElementChild).toHaveAttribute('data-ready', 'true')
    rerender(<PageContainer variant="wide">x</PageContainer>)
    expect(container.firstElementChild).toHaveAttribute('data-ready', 'false')
  })

  it('sets data-page from pageName', () => {
    const { container } = render(
      <PageContainer variant="fluid" pageName="board">
        x
      </PageContainer>,
    )
    expect(container.firstElementChild).toHaveAttribute('data-page', 'board')
  })

  it('does not render an aside landmark without a sidebar', () => {
    render(<PageContainer variant="reading">x</PageContainer>)
    expect(screen.queryByRole('complementary')).not.toBeInTheDocument()
  })

  it('renders the sidebar in an aside landmark, beside the content on wide viewports', () => {
    const { container } = render(
      <PageContainer variant="wide" sidebar={<p>Detalle</p>}>
        x
      </PageContainer>,
    )
    expect(screen.getByRole('complementary', { name: 'Panel lateral' })).toHaveTextContent('Detalle')
    expect(container.firstElementChild).toHaveAttribute('data-has-sidebar', 'true')
    expect(container.firstElementChild).toHaveAttribute('data-stacked', 'false')
  })

  it('moves the sidebar below the content under 992 px', () => {
    restore()
    restore = mockViewport(true)
    const { container } = render(
      <PageContainer variant="wide" sidebar={<p>Detalle</p>}>
        x
      </PageContainer>,
    )
    expect(container.firstElementChild).toHaveAttribute('data-stacked', 'true')
  })

  it('is not filled by default', () => {
    const { container } = render(<PageContainer variant="wide">x</PageContainer>)
    expect(container.firstElementChild).toHaveAttribute('data-fill', 'false')
  })

  it('marks the page as filled with fill, so the content column grows to the viewport', () => {
    const { container } = render(
      <PageContainer variant="wide" fill>
        <section>x</section>
      </PageContainer>,
    )
    expect(container.firstElementChild).toHaveAttribute('data-fill', 'true')
    expect(container.querySelector('section')?.parentElement?.className).toContain('content')
  })
})
