import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { FieldRow } from './FieldRow'

describe('FieldRow', () => {
  it('associates the label with the control', () => {
    render(<FieldRow label="Nombre">{(props) => <input {...props} />}</FieldRow>)
    expect(screen.getByRole('textbox', { name: 'Nombre' })).toBeInTheDocument()
  })

  it('marks required fields with aria-required and a spoken "(obligatorio)" hint', () => {
    render(<FieldRow label="Clave" required>{(props) => <input {...props} />}</FieldRow>)
    const input = screen.getByRole('textbox', { name: /Clave/ })
    expect(input).toHaveAttribute('aria-required', 'true')
    expect(screen.getByText('(obligatorio)')).toBeInTheDocument()
  })

  it('does not set aria-required on optional fields', () => {
    render(<FieldRow label="Descripción">{(props) => <input {...props} />}</FieldRow>)
    expect(screen.getByRole('textbox', { name: 'Descripción' })).not.toHaveAttribute('aria-required')
  })

  it('links helper text to the control through aria-describedby', () => {
    render(
      <FieldRow label="Clave de proyecto" helper="Tres letras mayúsculas">
        {(props) => <input {...props} />}
      </FieldRow>,
    )
    const input = screen.getByRole('textbox', { name: 'Clave de proyecto' })
    expect(input).toHaveAccessibleDescription('Tres letras mayúsculas')
  })

  it('shows the error instead of the helper and marks the control invalid', () => {
    render(
      <FieldRow label="Clave de proyecto" helper="Tres letras mayúsculas" error="La clave ya existe">
        {(props) => <input {...props} />}
      </FieldRow>,
    )
    const input = screen.getByRole('textbox', { name: 'Clave de proyecto' })
    expect(input).toHaveAttribute('aria-invalid', 'true')
    expect(input).toHaveAccessibleDescription('La clave ya existe')
    expect(screen.queryByText('Tres letras mayúsculas')).not.toBeInTheDocument()
  })

  it('accepts a plain node as control, without injected attributes', () => {
    render(
      <FieldRow label="Estado">
        <span>Activo</span>
      </FieldRow>,
    )
    expect(screen.getByText('Activo')).toBeInTheDocument()
  })

  it('exposes the variant as data-variant (stacked by default, inline for detail panels)', () => {
    const { container, rerender } = render(<FieldRow label="A">x</FieldRow>)
    expect(container.firstElementChild).toHaveAttribute('data-variant', 'stacked')
    rerender(<FieldRow label="A" variant="inline">x</FieldRow>)
    expect(container.firstElementChild).toHaveAttribute('data-variant', 'inline')
  })
})
