import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'

import { ErrorState } from './ErrorState'

describe('ErrorState', () => {
  it('announces the message as an alert with the default title', () => {
    render(<ErrorState message="Revisa tu conexión" />)
    expect(screen.getByRole('alert')).toBeInTheDocument()
    expect(screen.getByText('No se pudo cargar la información')).toBeInTheDocument()
    expect(screen.getByText('Revisa tu conexión')).toBeInTheDocument()
  })

  it('calls onRetry when the retry button is pressed', () => {
    const onRetry = vi.fn()
    render(<ErrorState message="Error" onRetry={onRetry} />)
    fireEvent.click(screen.getByRole('button', { name: 'Reintentar' }))
    expect(onRetry).toHaveBeenCalledTimes(1)
  })

  it('does not render a retry button without onRetry', () => {
    render(<ErrorState message="Error" />)
    expect(screen.queryByRole('button', { name: 'Reintentar' })).not.toBeInTheDocument()
  })
})
