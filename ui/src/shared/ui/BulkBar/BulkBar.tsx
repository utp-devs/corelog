import type { ReactNode } from 'react'
import { Button } from 'antd'

import styles from './BulkBar.module.css'

export type BulkAction = {
  key: string
  label: string
  icon?: ReactNode
  danger?: boolean
  onClick: () => void
}

type BulkBarProps = {
  /** Número de filas seleccionadas. La barra solo se muestra si es mayor que cero. */
  count: number
  actions: BulkAction[]
  /** Quita la selección (botón "Limpiar selección"). */
  onClear: () => void
}

/** Barra de acciones masivas: aparece sobre una tabla cuando hay filas seleccionadas. */
export function BulkBar({ count, actions, onClear }: BulkBarProps) {
  if (count <= 0) {
    return null
  }

  const countText = count === 1 ? '1 seleccionado' : `${count} seleccionados`

  return (
    <div role="region" aria-label="Acciones masivas" className={styles.bar}>
      <span className={styles.count} aria-live="polite">
        {countText}
      </span>
      <div className={styles.actions}>
        {actions.map((action) => (
          <Button
            key={action.key}
            size="small"
            icon={action.icon}
            danger={action.danger}
            onClick={action.onClick}
          >
            {action.label}
          </Button>
        ))}
        <Button size="small" type="text" onClick={onClear}>
          Limpiar selección
        </Button>
      </div>
    </div>
  )
}
