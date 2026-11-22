import type { ReactNode } from 'react'

import styles from './StatTile.module.css'

type StatTileProps = {
  label: string
  value: ReactNode
  /** Texto secundario bajo el valor, por ejemplo una variación o una unidad. */
  hint?: string
}

/** KPI compacto para paneles y reportes: etiqueta, valor destacado y dato de contexto opcional. */
export function StatTile({ label, value, hint }: StatTileProps) {
  return (
    <div className={styles.tile}>
      <span className={styles.label}>{label}</span>
      <span className={styles.value}>{value}</span>
      {hint && <span className={styles.hint}>{hint}</span>}
    </div>
  )
}
