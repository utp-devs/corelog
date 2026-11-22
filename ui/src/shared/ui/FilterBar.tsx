import { Button, Select, Space } from 'antd'
import type { ReactNode } from 'react'

import { SearchInput } from './SearchInput'
import styles from './FilterBar.module.css'

export type FilterOption = { value: string; label: string }

export type FilterDefinition = {
  key: string
  label: string
  options: FilterOption[]
  value?: string
  onChange: (value?: string) => void
}

type FilterBarProps = {
  search: { value: string; onChange: (value: string) => void; placeholder?: string }
  filters?: FilterDefinition[]
  /** Acciones adicionales a la derecha, por ejemplo "Guardar filtro". */
  actions?: ReactNode
  /** Limpia la búsqueda y los filtros. El botón solo aparece si hay algo activo. */
  onClear?: () => void
}

/** Barra de filtros: búsqueda con debounce, selectores por campo y limpieza. */
export function FilterBar({ search, filters = [], actions, onClear }: FilterBarProps) {
  const hasActive = search.value !== '' || filters.some((filter) => filter.value !== undefined && filter.value !== '')

  return (
    <div className={styles.bar}>
      <Space wrap size="small" className={styles.controls}>
        <div className={styles.search}>
          <SearchInput value={search.value} onChange={search.onChange} placeholder={search.placeholder} />
        </div>
        {filters.map((filter) => (
          <Select
            key={filter.key}
            allowClear
            placeholder={filter.label}
            aria-label={filter.label}
            className={styles.select}
            value={filter.value}
            options={filter.options}
            onChange={(value) => filter.onChange(value ?? undefined)}
          />
        ))}
        {onClear && hasActive && (
          <Button type="link" onClick={onClear}>
            Limpiar filtros
          </Button>
        )}
      </Space>
      {actions && <div className={styles.actions}>{actions}</div>}
    </div>
  )
}
