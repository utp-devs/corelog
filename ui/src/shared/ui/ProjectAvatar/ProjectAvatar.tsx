import type { ReactNode } from 'react'

import { paletteIndex } from '../Avatar/avatarColor'
import styles from './ProjectAvatar.module.css'

export type ProjectAvatarSize = 24 | 32 | 40

type ProjectAvatarProps = {
  /** Nombre del proyecto: nombre accesible. */
  name: string
  /** Clave del proyecto (por ejemplo CL): decide el color y, sin icono, las iniciales. */
  projectKey: string
  /** Icono opcional, por ejemplo un icono de @ant-design/icons. */
  icon?: ReactNode
  size?: ProjectAvatarSize
  'data-testid'?: string
}

/** Cuadrado de color con icono o iniciales de la clave del proyecto. */
export function ProjectAvatar({ name, projectKey, icon, size = 32, 'data-testid': testId }: ProjectAvatarProps) {
  const initials = projectKey.trim().toUpperCase().slice(0, 2)
  return (
    <span
      role="img"
      aria-label={`Proyecto ${name}`}
      className={`${styles.tile} ${styles[`size${size}`]}`}
      data-palette={paletteIndex(projectKey)}
      data-testid={testId}
    >
      <span aria-hidden="true" className={styles.content}>
        {icon ?? initials}
      </span>
    </span>
  )
}
