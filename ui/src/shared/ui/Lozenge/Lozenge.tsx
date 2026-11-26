import type { ReactNode } from 'react'

import styles from './Lozenge.module.css'

export type LozengeCategory = 'todo' | 'progress' | 'done'
export type LozengeTone = 'info' | 'success' | 'warning' | 'danger' | 'neutral'

type LozengeProps = {
  /** Texto visible: siempre se muestra, nunca solo color. */
  children: ReactNode
  /** Categoría de estado del flujo de trabajo. Tiene prioridad sobre tone. */
  category?: LozengeCategory
  tone?: LozengeTone
  'data-testid'?: string
}

/** Chip compacto para estado o tono semántico. */
export function Lozenge({ children, category, tone = 'neutral', 'data-testid': testId }: LozengeProps) {
  const variant: LozengeCategory | LozengeTone = category ?? tone
  return (
    <span className={`${styles.lozenge} ${styles[variant]}`} data-variant={variant} data-testid={testId}>
      {children}
    </span>
  )
}
