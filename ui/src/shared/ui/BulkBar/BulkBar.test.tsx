import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'

import { BulkBar } from './BulkBar'

describe('BulkBar', () => {
  it('renders nothing when no row is selected', () => {
    render(<BulkBar count={0} actions={[]} onClear={() => {}} />)
    expect(screen.queryByRole('region', { name: 'Acciones masivas' })).not.toBeInTheDocument()
  })

  it('shows the count, the actions and the clear button when rows are selected', () => {
    const onDelete = vi.fn()
    const onClear = vi.fn()
    render(
      <BulkBar
        count={3}
        actions={[{ key: 'delete', label: 'Eliminar', danger: true, onClick: onDelete }]}
        onClear={onClear}
      />,
    )

    const region = screen.getByRole('region', { name: 'Acciones masivas' })
    expect(region).toHaveTextContent('3 seleccionados')
    expect(screen.getByText('3 seleccionados')).toHaveAttribute('aria-live', 'polite')

    fireEvent.click(screen.getByRole('button', { name: 'Eliminar' }))
    expect(onDelete).toHaveBeenCalledTimes(1)

    fireEvent.click(screen.getByRole('button', { name: 'Limpiar selección' }))
    expect(onClear).toHaveBeenCalledTimes(1)
  })

  it('uses the singular form for one selected row', () => {
    render(<BulkBar count={1} actions={[]} onClear={() => {}} />)
    expect(screen.getByText('1 seleccionado')).toBeInTheDocument()
  })
})
