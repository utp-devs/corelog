import type { HTMLAttributes, KeyboardEvent, MouseEvent, ReactNode } from 'react'
import { Button, Checkbox, Dropdown, Pagination, Segmented, Table } from 'antd'
import type { MenuProps } from 'antd'
import type { ColumnType, ColumnsType } from 'antd/es/table'
import { EllipsisOutlined, SettingOutlined } from '@ant-design/icons'

import { type BulkAction, BulkBar } from '../BulkBar/BulkBar'
import { EmptyState } from '../EmptyState'
import { ErrorState } from '../ErrorState'
import styles from './DataTable.module.css'
import { useDelayedFlag } from './useDelayedFlag'
import { useMediaQuery } from './useMediaQuery'
import { usePersistentState } from './usePersistentState'

/** Columna de antd con las opciones propias del DataTable. `key` es obligatoria: identifica la columna al persistirla. */
export type DataTableColumn<T> = ColumnType<T> & {
  key: string
  /** Aparece en el selector de columnas. */
  hideable?: boolean
  /** Oculta por defecto (solo si el usuario no ha guardado su propia elección). */
  defaultHidden?: boolean
  /** Visible en móvil cuando no hay `mobileRender`: solo se muestran estas columnas. */
  mobilePrimary?: boolean
}

/** Elemento del menú de acciones de fila (formato de antd `MenuProps['items']`). */
export type RowMenuItem = NonNullable<MenuProps['items']>[number]

export type Density = 'comfortable' | 'compact'

export type DataTablePagination = {
  /** Página actual, empieza en 1. */
  page: number
  pageSize: number
  total: number
  /** Paginación en servidor: el componente nunca pagina datos en cliente. */
  onChange: (page: number, pageSize: number) => void
}

export type DataTableSelection = {
  selectedKeys: string[]
  onChange: (keys: string[]) => void
  actions: BulkAction[]
}

export type DataTableProps<T extends object> = {
  /** Clave de persistencia del selector de columnas (`corelog.table.<tableKey>.columns`). */
  tableKey: string
  columns: DataTableColumn<T>[]
  data: T[]
  /** Nombre de la propiedad identificadora o función que devuelve la clave de la fila. */
  rowKey: string | ((row: T) => string)
  loading?: boolean
  /** Mensaje de error: sustituye la tabla por `ErrorState` con reintento. */
  error?: string | null
  onRetry?: () => void
  /** Contenido de estado vacío (máximo 280 px). Por defecto, `EmptyState`. */
  emptyState?: ReactNode
  pagination?: DataTablePagination
  selection?: DataTableSelection
  /** Abre la fila con clic o con Enter sobre la fila enfocada. Las filas reciben foco con el teclado. */
  onRowOpen?: (row: T) => void
  /** Acciones de la fila, mostradas en un menú "...". */
  rowActions?: (row: T) => RowMenuItem[]
  /** Por debajo de 768 px sustituye la tabla por una lista de tarjetas. */
  mobileRender?: (row: T) => ReactNode
}

const PAGE_SIZE_OPTIONS = [20, 50, 100]
const SKELETON_MAX_ROWS = 10
const SKELETON_DEFAULT_ROWS = 5
const SKELETON_DELAY_MS = 150
const MOBILE_QUERY = '(max-width: 767px)'
/** Altura de la barra superior fija (--cl-topbar-h), usada como offset del encabezado pegajoso. */
const TOPBAR_OFFSET = 56
const DENSITY_STORAGE_KEY = 'corelog.density'
const ROW_CLASS = 'cl-data-row'
const INTERACTIVE_SELECTOR = 'button, a, input, label, select, textarea, [role="menuitem"]'

type SkeletonRow = { index: number }

function isDensity(value: unknown): value is Density {
  return value === 'comfortable' || value === 'compact'
}

function isInteractiveTarget(target: EventTarget): boolean {
  return target instanceof Element && target.closest(INTERACTIVE_SELECTOR) !== null
}

function moveRowFocus(current: HTMLElement, step: 1 | -1) {
  let sibling = step === 1 ? current.nextElementSibling : current.previousElementSibling
  while (sibling && !sibling.classList.contains(ROW_CLASS)) {
    sibling = step === 1 ? sibling.nextElementSibling : sibling.previousElementSibling
  }
  if (sibling instanceof HTMLElement) {
    sibling.focus()
  }
}

/** Teclado sobre una fila enfocada: flechas arriba/abajo mueven el foco, Enter abre. */
function handleRowKeyDown<T>(event: KeyboardEvent<HTMLElement>, row: T, onOpen: (row: T) => void) {
  if (event.target !== event.currentTarget) {
    return
  }
  if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
    event.preventDefault()
    moveRowFocus(event.currentTarget, event.key === 'ArrowDown' ? 1 : -1)
  } else if (event.key === 'Enter') {
    event.preventDefault()
    onOpen(row)
  }
}

function headerOf<T>(column: DataTableColumn<T>): ReactNode {
  return typeof column.title === 'function' ? null : column.title
}

function labelOf<T>(column: DataTableColumn<T>): string {
  return typeof column.title === 'string' ? column.title : column.key
}

/** Tabla de datos sobre antd Table: cabecera fija, densidad, columnas persistidas, selección con BulkBar, estados de carga, vacío y error, paginación del servidor, modo tarjetas en móvil, acciones de fila y navegación por teclado. */
export function DataTable<T extends object>({
  tableKey,
  columns,
  data,
  rowKey,
  loading = false,
  error,
  onRetry,
  emptyState,
  pagination,
  selection,
  onRowOpen,
  rowActions,
  mobileRender,
}: DataTableProps<T>) {
  const keyOf = (row: T): string =>
    typeof rowKey === 'function' ? rowKey(row) : String((row as Record<string, unknown>)[rowKey])

  const [storedDensity, setDensity] = usePersistentState<Density>(DENSITY_STORAGE_KEY, 'comfortable')
  const density: Density = isDensity(storedDensity) ? storedDensity : 'comfortable'

  const defaultHidden = columns.filter((column) => column.defaultHidden).map((column) => column.key)
  const [hiddenKeys, setHiddenKeys] = usePersistentState<string[]>(
    `corelog.table.${tableKey}.columns`,
    defaultHidden,
  )

  const isMobile = useMediaQuery(MOBILE_QUERY)
  const cardMode = isMobile && mobileRender !== undefined
  const primaryOnly = isMobile && !cardMode && columns.some((column) => column.mobilePrimary)
  const showSkeleton = useDelayedFlag(loading, SKELETON_DELAY_MS)
  const skeletonRowCount = Math.min(pagination?.pageSize ?? SKELETON_DEFAULT_ROWS, SKELETON_MAX_ROWS)

  const visibleColumns = columns.filter((column) => {
    if (primaryOnly) {
      return column.mobilePrimary === true
    }
    return !(column.hideable && hiddenKeys.includes(column.key))
  })

  const selectedKeys = selection?.selectedKeys ?? []

  function toggleSelectedKey(key: string, checked: boolean) {
    if (!selection) {
      return
    }
    const next = checked ? [...selectedKeys, key] : selectedKeys.filter((item) => item !== key)
    selection.onChange(next)
  }

  function toggleColumn(key: string, visible: boolean) {
    setHiddenKeys(visible ? hiddenKeys.filter((item) => item !== key) : [...hiddenKeys, key])
  }

  function rowInteraction(row: T): HTMLAttributes<HTMLElement> {
    if (!onRowOpen) {
      return { className: ROW_CLASS }
    }
    return {
      className: ROW_CLASS,
      tabIndex: 0,
      onClick: (event: MouseEvent<HTMLElement>) => {
        if (!isInteractiveTarget(event.target)) {
          onRowOpen(row)
        }
      },
      onKeyDown: (event: KeyboardEvent<HTMLElement>) => handleRowKeyDown(event, row, onRowOpen),
    }
  }

  function renderRowActions(row: T): ReactNode {
    if (!rowActions) {
      return null
    }
    return (
      <Dropdown menu={{ items: rowActions(row) }} trigger={['click']} placement="bottomRight">
        <Button type="text" size="small" icon={<EllipsisOutlined />} aria-label="Acciones de la fila" />
      </Dropdown>
    )
  }

  const tableColumns: ColumnsType<T> = visibleColumns.map((column) => column)
  if (rowActions && !cardMode) {
    tableColumns.push({
      key: '__row-actions',
      title: '',
      width: 56,
      align: 'right',
      render: (_value, row) => renderRowActions(row),
    })
  }

  const skeletonColumns: ColumnsType<SkeletonRow> = visibleColumns.map((column) => ({
    key: column.key,
    title: headerOf(column),
    align: column.align,
    width: column.width,
    render: () => <span className={styles.skeletonBar} aria-hidden="true" />,
  }))
  const skeletonData: SkeletonRow[] = Array.from({ length: skeletonRowCount }, (_, index) => ({ index }))

  const emptyContent = (
    <div className={styles.empty}>
      {emptyState ?? <EmptyState title="Sin resultados" description="No hay elementos que mostrar." />}
    </div>
  )

  const rowSelection = selection
    ? {
        selectedRowKeys: selectedKeys,
        onChange: (keys: React.Key[]) => selection.onChange(keys.map(String)),
        columnWidth: 48,
      }
    : undefined

  let body: ReactNode
  if (error) {
    body = <ErrorState message={error} onRetry={onRetry} />
  } else if (showSkeleton) {
    body = (
      <div role="status" aria-busy="true" aria-label="Cargando">
        <Table<SkeletonRow>
          rowKey="index"
          columns={skeletonColumns}
          dataSource={skeletonData}
          pagination={false}
          sticky={{ offsetHeader: TOPBAR_OFFSET }}
        />
      </div>
    )
  } else if (cardMode && mobileRender) {
    body =
      data.length === 0 && !loading ? (
        emptyContent
      ) : (
        <ul className={styles.cards}>
          {data.map((row) => (
            <li key={keyOf(row)} className={styles.card} {...rowInteraction(row)}>
              {selection && (
                <Checkbox
                  checked={selectedKeys.includes(keyOf(row))}
                  onChange={(event) => toggleSelectedKey(keyOf(row), event.target.checked)}
                  aria-label="Seleccionar fila"
                />
              )}
              <div className={styles.cardBody}>{mobileRender(row)}</div>
              {renderRowActions(row)}
            </li>
          ))}
        </ul>
      )
  } else {
    body = (
      <Table<T>
        rowKey={keyOf}
        columns={tableColumns}
        dataSource={data}
        pagination={false}
        sticky={{ offsetHeader: TOPBAR_OFFSET }}
        rowSelection={rowSelection}
        onRow={rowInteraction}
        locale={{ emptyText: loading ? <span /> : emptyContent }}
      />
    )
  }

  const rootClassName = [
    styles.root,
    density === 'compact' ? styles.compact : styles.comfortable,
    onRowOpen ? styles.clickable : '',
  ]
    .filter(Boolean)
    .join(' ')

  return (
    <div className={rootClassName} aria-busy={loading || undefined}>
      <div className={styles.toolbar}>
        {selection && (
          <BulkBar
            count={selectedKeys.length}
            actions={selection.actions}
            onClear={() => selection.onChange([])}
          />
        )}
        {!cardMode && (
          <div className={styles.tools}>
            {columns.some((column) => column.hideable) && (
              <Dropdown
                trigger={['click']}
                placement="bottomRight"
                popupRender={() => (
                  <div className={styles.chooser} role="group" aria-label="Columnas visibles">
                    {columns
                      .filter((column) => column.hideable)
                      .map((column) => (
                        <Checkbox
                          key={column.key}
                          checked={!hiddenKeys.includes(column.key)}
                          onChange={(event) => toggleColumn(column.key, event.target.checked)}
                        >
                          {labelOf(column)}
                        </Checkbox>
                      ))}
                  </div>
                )}
              >
                <Button size="small" icon={<SettingOutlined />}>
                  Columnas
                </Button>
              </Dropdown>
            )}
            <div role="group" aria-label="Densidad">
              <Segmented
                size="small"
                value={density}
                onChange={(value) => setDensity(isDensity(value) ? value : 'comfortable')}
                options={[
                  { label: 'Cómoda', value: 'comfortable' },
                  { label: 'Compacta', value: 'compact' },
                ]}
              />
            </div>
          </div>
        )}
      </div>

      {body}

      {pagination && (
        <div className={styles.footer}>
          <Pagination
            current={pagination.page}
            pageSize={pagination.pageSize}
            total={pagination.total}
            pageSizeOptions={PAGE_SIZE_OPTIONS}
            showSizeChanger
            showTotal={(total) => `${total} elementos`}
            onChange={(page, pageSize) => pagination.onChange(page, pageSize)}
          />
        </div>
      )}
    </div>
  )
}
