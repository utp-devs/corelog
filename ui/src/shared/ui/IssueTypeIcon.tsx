import type { ReactElement } from 'react'
import { ApartmentOutlined, BookOutlined, BugOutlined, CheckSquareOutlined, ThunderboltOutlined } from '@ant-design/icons'

import type { IssueTypeKind } from '@/shared/types/issue'

import styles from './IssueTypeIcon.module.css'

const typeMap: Record<IssueTypeKind, { label: string; icon: ReactElement; className: string }> = {
  task: { label: 'Tarea', icon: <CheckSquareOutlined />, className: styles.task },
  bug: { label: 'Error', icon: <BugOutlined />, className: styles.bug },
  story: { label: 'Historia', icon: <BookOutlined />, className: styles.story },
  epic: { label: 'Épica', icon: <ThunderboltOutlined />, className: styles.epic },
  subtask: { label: 'Subtarea', icon: <ApartmentOutlined />, className: styles.subtask },
}

type IssueTypeIconProps = {
  type: IssueTypeKind
  /** Muestra el nombre del tipo junto al icono. */
  showLabel?: boolean
}

/** Icono con color propio por tipo de issue. El nombre queda accesible para lectores de pantalla. */
export function IssueTypeIcon({ type, showLabel = false }: IssueTypeIconProps) {
  const { label, icon, className } = typeMap[type]
  return (
    <span className={`${styles.wrapper} ${className}`} title={label}>
      <span aria-hidden="true">{icon}</span>
      {showLabel ? <span>{label}</span> : <span className={styles.srOnly}>{label}</span>}
    </span>
  )
}
