import { render, screen, within } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { Section } from './Section'

describe('Section', () => {
  it('renders the title as a level 2 heading that names the region', () => {
    render(<Section title="Detalles">contenido</Section>)
    const region = screen.getByRole('region', { name: 'Detalles' })
    expect(within(region).getByRole('heading', { level: 2, name: 'Detalles' })).toBeInTheDocument()
  })

  it('honours headingLevel', () => {
    render(
      <Section title="Subbloque" headingLevel={4}>
        x
      </Section>,
    )
    expect(screen.getByRole('heading', { level: 4, name: 'Subbloque' })).toBeInTheDocument()
  })

  it('renders description, actions and footer when provided', () => {
    render(
      <Section
        title="Miembros"
        description="Personas con acceso a la organización"
        actions={<button type="button">Invitar</button>}
        footer={<span>Total: 3</span>}
      >
        x
      </Section>,
    )
    expect(screen.getByText('Personas con acceso a la organización')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Invitar' })).toBeInTheDocument()
    expect(screen.getByText('Total: 3')).toBeInTheDocument()
  })

  it('uses ariaLabel as the region name when there is no title', () => {
    render(<Section ariaLabel="Resumen">x</Section>)
    expect(screen.getByRole('region', { name: 'Resumen' })).toBeInTheDocument()
    expect(screen.queryByRole('heading')).not.toBeInTheDocument()
  })

  it('omits the header and footer when not provided', () => {
    const { container } = render(<Section>x</Section>)
    expect(container.querySelector('header')).toBeNull()
    expect(container.querySelector('footer')).toBeNull()
  })

  it('pads the body by default and removes the padding with padded={false} or flush', () => {
    const { container, rerender } = render(<Section>x</Section>)
    const body = () => container.querySelector('section > div') as HTMLElement
    expect(body().className).toContain('body')
    expect(body().className).not.toContain('bodyFlush')

    rerender(<Section padded={false}>x</Section>)
    expect(body().className).toContain('bodyFlush')

    rerender(<Section flush>x</Section>)
    expect(body().className).toContain('bodyFlush')
  })

  it('is not filled by default', () => {
    const { container } = render(<Section>x</Section>)
    const section = container.querySelector('section') as HTMLElement
    expect(section).toHaveAttribute('data-fill', 'false')
    expect((section.firstElementChild as HTMLElement).className).not.toContain('grow')
  })

  it('grows with fill and stretches its body to the remaining height', () => {
    const { container } = render(
      <Section title="Miembros" fill>
        x
      </Section>,
    )
    const section = container.querySelector('section') as HTMLElement
    expect(section).toHaveAttribute('data-fill', 'true')
    expect((section.lastElementChild as HTMLElement).className).toContain('grow')
  })
})
