import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { ProgressBar } from './ProgressBar'

describe('ProgressBar', () => {
  it('exposes the value to assistive technology with the label as name', () => {
    render(<ProgressBar value={40} label="Completado" />)
    const bar = screen.getByRole('progressbar', { name: 'Completado' })
    expect(bar).toHaveAttribute('aria-valuenow', '40')
    expect(bar).toHaveAttribute('aria-valuemin', '0')
    expect(bar).toHaveAttribute('aria-valuemax', '100')
    expect(screen.getByText('40%')).toBeInTheDocument()
  })

  it('clamps values outside 0-100 and non-finite values', () => {
    const { unmount } = render(<ProgressBar value={150} label="Alto" />)
    expect(screen.getByRole('progressbar', { name: 'Alto' })).toHaveAttribute('aria-valuenow', '100')
    unmount()
    const { unmount: unmountLow } = render(<ProgressBar value={-5} label="Bajo" />)
    expect(screen.getByRole('progressbar', { name: 'Bajo' })).toHaveAttribute('aria-valuenow', '0')
    unmountLow()
    render(<ProgressBar value={Number.NaN} label="Nulo" />)
    expect(screen.getByRole('progressbar', { name: 'Nulo' })).toHaveAttribute('aria-valuenow', '0')
  })
})
