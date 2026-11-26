import type { ReactNode } from 'react'

import styles from './EmptyState.module.css'

type EmptyStateProps = {
  title: string
  description?: string
  icon?: ReactNode
  action?: ReactNode
}

/** Estado vacío de una lista, tabla o panel: icono, título, descripción y acción opcional. */
export function EmptyState({ title, description, icon, action }: EmptyStateProps) {
  return (
    <div className={styles.container} role="status" data-empty-state="true">
      {icon && <div className={styles.icon} aria-hidden="true">{icon}</div>}
      <p className={styles.title}>{title}</p>
      {description && <p className={styles.description}>{description}</p>}
      {action && <div className={styles.action}>{action}</div>}
    </div>
  )
}
