import { fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { DataTable, type DataTableColumn, type DataTableProps } from './DataTable'

type Row = { id: string; title: string; createdAt: string }

const rows: Row[] = [
  { id: '1', title: 'Fila 1', createdAt: '2026-11-01' },
  { id: '2', title: 'Fila 2', createdAt: '2026-11-02' },
  { id: '3', title: 'Fila 3', createdAt: '2026-11-03' },
]

const columns: DataTableColumn<Row>[] = [
  { key: 'title', title: 'Título', dataIndex: 'title', mobilePrimary: true },
  { key: 'createdAt', title: 'Creado', dataIndex: 'createdAt', hideable: true, align: 'right' },
]

function renderTable(overrides: Partial<DataTableProps<Row>> = {}) {
  const props: DataTableProps<Row> = {
    tableKey: 'test-table',
    columns,
    data: rows,
    rowKey: 'id',
    ...overrides,
  }
  return render(<DataTable {...props} />)
}

const originalMatchMedia = window.matchMedia

beforeEach(() => {
  window.localStorage.clear()
})

afterEach(() => {
  window.matchMedia = originalMatchMedia
})

describe('DataTable', () => {
  it('shows skeleton rows after the loading delay', async () => {
    renderTable({ data: [], loading: true, pagination: { page: 1, pageSize: 20, total: 0, onChange: () => {} } })

    expect(screen.queryByRole('status', { name: 'Cargando' })).not.toBeInTheDocument()

    const loadingRegion = await screen.findByRole('status', { name: 'Cargando' }, { timeout: 1000 })
    // Page size 20 is capped at 10 skeleton rows.
    expect(within(loadingRegion).getAllByRole('row')).toHaveLength(11)
  })

  it('renders the default empty state or a custom empty slot', () => {
    const { unmount } = renderTable({ data: [] })
    expect(screen.getByText('Sin resultados')).toBeInTheDocument()
    unmount()

    renderTable({ data: [], emptyState: <p>Aún no hay filas</p> })
    expect(screen.getByText('Aún no hay filas')).toBeInTheDocument()
  })

  it('replaces the table with an error and calls onRetry', () => {
    const onRetry = vi.fn()
    renderTable({ error: 'Sin conexión con el servidor', onRetry })

    expect(screen.getByRole('alert')).toHaveTextContent('Sin conexión con el servidor')
    expect(screen.queryByRole('table')).not.toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Reintentar' }))
    expect(onRetry).toHaveBeenCalledTimes(1)
  })

  it('selects rows and shows BulkBar actions that can clear the selection', () => {
    const onChange = vi.fn()
    const onDelete = vi.fn()
    renderTable({
      selection: {
        selectedKeys: ['1'],
        onChange,
        actions: [{ key: 'delete', label: 'Eliminar', danger: true, onClick: onDelete }],
      },
    })

    const region = screen.getByRole('region', { name: 'Acciones masivas' })
    expect(within(region).getByText('1 seleccionado')).toBeInTheDocument()

    fireEvent.click(within(region).getByRole('button', { name: 'Eliminar' }))
    expect(onDelete).toHaveBeenCalledTimes(1)

    // Row checkboxes: index 0 is the header "select all", index 2 is row 2.
    const checkboxes = screen.getAllByRole('checkbox')
    fireEvent.click(checkboxes[2])
    expect(onChange).toHaveBeenLastCalledWith(['1', '2'])

    fireEvent.click(within(region).getByRole('button', { name: 'Limpiar selección' }))
    expect(onChange).toHaveBeenLastCalledWith([])
  })

  it('calls onChange with page and page size from the footer', () => {
    const onChange = vi.fn()
    renderTable({ pagination: { page: 1, pageSize: 20, total: 100, onChange } })

    expect(screen.getByText('100 elementos')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: /next page/i }))
    expect(onChange).toHaveBeenLastCalledWith(2, 20)
  })

  it('changes the page size from the footer selector (20, 50, 100)', async () => {
    const onChange = vi.fn()
    renderTable({ pagination: { page: 1, pageSize: 20, total: 100, onChange } })

    fireEvent.mouseDown(screen.getByRole('combobox'))
    fireEvent.click(await screen.findByText('50 / page'))
    expect(onChange).toHaveBeenLastCalledWith(1, 50)
  })

  it('persists the density choice in corelog.density across remounts', () => {
    const { unmount } = renderTable()
    fireEvent.click(screen.getByRole('radio', { name: 'Compacta' }))
    expect(window.localStorage.getItem('corelog.density')).toBe('"compact"')
    unmount()

    renderTable()
    expect(screen.getByRole('radio', { name: 'Compacta' })).toBeChecked()
  })

  it('persists hidden columns per tableKey across remounts', () => {
    const { unmount } = renderTable()
    fireEvent.click(screen.getByRole('button', { name: /Columnas/ }))
    fireEvent.click(screen.getByRole('checkbox', { name: 'Creado' }))
    expect(screen.queryByRole('columnheader', { name: 'Creado' })).not.toBeInTheDocument()
    unmount()

    renderTable()
    expect(screen.queryByRole('columnheader', { name: 'Creado' })).not.toBeInTheDocument()
    expect(window.localStorage.getItem('corelog.table.test-table.columns')).toBe('["createdAt"]')
  })

  it('renders cards instead of the table below 768 px when mobileRender is set', () => {
    window.matchMedia = vi.fn((query: string) => ({
      matches: query === '(max-width: 767px)',
      media: query,
      onchange: null,
      addEventListener: () => {},
      removeEventListener: () => {},
      addListener: () => {},
      removeListener: () => {},
      dispatchEvent: () => false,
    })) as unknown as typeof window.matchMedia

    renderTable({ mobileRender: (row) => <span>Tarjeta {row.title}</span> })

    expect(screen.queryByRole('table')).not.toBeInTheDocument()
    expect(screen.getByText('Tarjeta Fila 2')).toBeInTheDocument()
  })

  it('moves focus with arrow keys and opens a row with Enter', () => {
    const onRowOpen = vi.fn()
    renderTable({ onRowOpen })

    const firstRow = screen.getByText('Fila 1').closest('tr') as HTMLElement
    const secondRow = screen.getByText('Fila 2').closest('tr') as HTMLElement
    firstRow.focus()

    fireEvent.keyDown(firstRow, { key: 'ArrowDown' })
    expect(secondRow).toHaveFocus()

    fireEvent.keyDown(secondRow, { key: 'ArrowUp' })
    expect(firstRow).toHaveFocus()

    fireEvent.keyDown(firstRow, { key: 'Enter' })
    expect(onRowOpen).toHaveBeenCalledWith(rows[0])

    fireEvent.click(secondRow.querySelector('td') as HTMLElement)
    expect(onRowOpen).toHaveBeenLastCalledWith(rows[1])
  })

  it('opens the row action menu and runs the chosen action', async () => {
    const onEdit = vi.fn()
    renderTable({
      rowActions: (row) => [{ key: 'edit', label: `Editar ${row.title}`, onClick: onEdit }],
    })

    fireEvent.click(screen.getAllByRole('button', { name: 'Acciones de la fila' })[0])
    fireEvent.click(await screen.findByRole('menuitem', { name: 'Editar Fila 1' }))
    expect(onEdit).toHaveBeenCalledTimes(1)
  })
})

describe('DataTable usage notes', () => {
  it('waits for the loading delay before the skeleton is removed when data arrives', async () => {
    const { rerender } = render(
      <DataTable tableKey="test-table" columns={columns} data={[]} rowKey="id" loading />,
    )
    await waitFor(() => expect(screen.getByRole('status', { name: 'Cargando' })).toBeInTheDocument())

    rerender(<DataTable tableKey="test-table" columns={columns} data={rows} rowKey="id" loading={false} />)
    await waitFor(() => expect(screen.queryByRole('status', { name: 'Cargando' })).not.toBeInTheDocument())
    expect(screen.getByText('Fila 3')).toBeInTheDocument()
  })
})
