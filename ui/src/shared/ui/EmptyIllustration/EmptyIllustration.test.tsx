import { render } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { EmptyIllustration, type EmptyIllustrationKind } from './EmptyIllustration'

const kinds: EmptyIllustrationKind[] = ['projects', 'issues', 'search', 'notes', 'generic']

describe('EmptyIllustration', () => {
  it.each(kinds)('renders the %s drawing as a decorative svg', (kind) => {
    const { container } = render(<EmptyIllustration kind={kind} />)
    const svg = container.querySelector('svg')
    expect(svg).not.toBeNull()
    expect(svg).toHaveAttribute('aria-hidden', 'true')
    expect(svg).toHaveAttribute('data-kind', kind)
    expect(svg?.querySelectorAll('path, rect, circle').length).toBeGreaterThan(0)
  })

  it.each(kinds)('uses no hex or rgb literals in the %s drawing', (kind) => {
    const { container } = render(<EmptyIllustration kind={kind} />)
    expect(container.innerHTML).not.toMatch(/#[0-9a-fA-F]{3,6}\b|rgba?\(/)
  })
})
