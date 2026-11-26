import type { CSSProperties } from 'react'

import styles from './ProgressBar.module.css'

type ProgressTone = 'brand' | 'success' | 'warning' | 'danger'

type ProgressBarProps = {
  /** Porcentaje 0-100; valores fuera de rango se acotan. */
  value: number
  /** Texto visible y nombre accesible, por ejemplo "Completado". */
  label: string
  tone?: ProgressTone
  'data-testid'?: string
}

function clampPercent(value: number): number {
  if (!Number.isFinite(value)) return 0
  return Math.min(100, Math.max(0, Math.round(value)))
}

/** Barra de progreso accesible. El ancho se pasa como propiedad CSS. */
export function ProgressBar({ value, label, tone = 'brand', 'data-testid': testId }: ProgressBarProps) {
  const percent = clampPercent(value)

  return (
    <div className={styles.wrapper} data-testid={testId}>
      <div className={styles.header}>
        <span>{label}</span>
        <span>{percent}%</span>
      </div>
      <div
        role="progressbar"
        aria-label={label}
        aria-valuemin={0}
        aria-valuemax={100}
        aria-valuenow={percent}
        className={styles.track}
      >
        {/* runtime-style */}
        <div className={`${styles.fill} ${styles[tone]}`} style={{ '--cl-progress': `${percent}%` } as CSSProperties} />
      </div>
    </div>
  )
}
