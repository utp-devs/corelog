import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'

import { ColorPicker } from './ColorPicker'
import { isHexColor, nextPaletteIndex, paletteIndexOf } from './colorValue'

describe('ColorPicker', () => {
  it('renders 12 palette swatches inside a named radio group', () => {
    render(<ColorPicker label="Color de la etiqueta" value="palette-3" onChange={() => {}} />)
    const group = screen.getByRole('radiogroup', { name: 'Color de la etiqueta' })
    expect(group).toBeInTheDocument()
    expect(screen.getAllByRole('radio')).toHaveLength(12)
    expect(screen.getByRole('radio', { name: 'Color 3' })).toHaveAttribute('aria-checked', 'true')
  })

  it('moves the selection with the arrow keys', () => {
    const onChange = vi.fn()
    render(<ColorPicker label="Color" value="palette-1" onChange={onChange} />)
    fireEvent.keyDown(screen.getByRole('radio', { name: 'Color 1' }), { key: 'ArrowRight' })
    expect(onChange).toHaveBeenLastCalledWith('palette-2')
    fireEvent.keyDown(screen.getByRole('radio', { name: 'Color 1' }), { key: 'ArrowLeft' })
    expect(onChange).toHaveBeenLastCalledWith('palette-12')
    fireEvent.keyDown(screen.getByRole('radio', { name: 'Color 1' }), { key: 'End' })
    expect(onChange).toHaveBeenLastCalledWith('palette-12')
  })

  it('keeps only the selected swatch in the tab order', () => {
    render(<ColorPicker label="Color" value="palette-5" onChange={() => {}} />)
    expect(screen.getByRole('radio', { name: 'Color 5' })).toHaveAttribute('tabindex', '0')
    expect(screen.getByRole('radio', { name: 'Color 1' })).toHaveAttribute('tabindex', '-1')
  })

  it('accepts a valid custom hex value', () => {
    const onChange = vi.fn()
    render(<ColorPicker label="Color" value={null} onChange={onChange} />)
    fireEvent.change(screen.getByLabelText('Color personalizado (hex)'), { target: { value: '#1a2B3c' } })
    expect(onChange).toHaveBeenCalledWith('#1a2B3c')
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('shows an error and does not emit an invalid hex value', () => {
    const onChange = vi.fn()
    render(<ColorPicker label="Color" value={null} onChange={onChange} />)
    fireEvent.change(screen.getByLabelText('Color personalizado (hex)'), { target: { value: '#12' } })
    expect(onChange).not.toHaveBeenCalled()
    expect(screen.getByRole('alert')).toHaveTextContent('hexadecimal')
  })

  it('hides the custom input when allowCustom is false', () => {
    render(<ColorPicker label="Color" value={null} onChange={() => {}} allowCustom={false} />)
    expect(screen.queryByLabelText('Color personalizado (hex)')).not.toBeInTheDocument()
  })
})

describe('colorValue helpers', () => {
  it('validates hex colors', () => {
    expect(isHexColor('#0A1b2C')).toBe(true)
    expect(isHexColor('#abc')).toBe(false)
    expect(isHexColor('red')).toBe(false)
  })

  it('reads palette indexes only inside 1..12', () => {
    expect(paletteIndexOf('palette-12')).toBe(12)
    expect(paletteIndexOf('palette-13')).toBeNull()
    expect(paletteIndexOf('#0A1B2C')).toBeNull()
    expect(paletteIndexOf(null)).toBeNull()
  })

  it('wraps navigation keys around the palette', () => {
    expect(nextPaletteIndex('ArrowDown', 12)).toBe(1)
    expect(nextPaletteIndex('ArrowUp', 1)).toBe(12)
    expect(nextPaletteIndex('Enter', 4)).toBeNull()
  })
})
