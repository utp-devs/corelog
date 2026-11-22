import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'

import { FilterBar } from './FilterBar'

const statusFilter = {
  key: 'status',
  label: 'Estado',
  options: [{ value: 'open', label: 'Abierto' }],
  onChange: () => {},
}

describe('FilterBar', () => {
  it('renders the search field and each filter selector', () => {
    render(<FilterBar search={{ value: '', onChange: () => {} }} filters={[statusFilter]} />)
    expect(screen.getByRole('textbox', { name: 'Buscar' })).toBeInTheDocument()
    expect(screen.getByRole('combobox', { name: 'Estado' })).toBeInTheDocument()
  })

  it('shows the clear action only when a filter is active', () => {
    const onClear = vi.fn()
    const { unmount } = render(
      <FilterBar search={{ value: '', onChange: () => {} }} filters={[statusFilter]} onClear={onClear} />,
    )
    expect(screen.queryByRole('button', { name: 'Limpiar filtros' })).not.toBeInTheDocument()
    unmount()

    render(<FilterBar search={{ value: 'login', onChange: () => {} }} onClear={onClear} />)
    fireEvent.click(screen.getByRole('button', { name: 'Limpiar filtros' }))
    expect(onClear).toHaveBeenCalledTimes(1)
  })

  it('renders the extra actions slot', () => {
    render(
      <FilterBar
        search={{ value: '', onChange: () => {} }}
        actions={<button type="button">Guardar filtro</button>}
      />,
    )
    expect(screen.getByRole('button', { name: 'Guardar filtro' })).toBeInTheDocument()
  })
})
