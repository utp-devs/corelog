import { LockOutlined, QuestionCircleOutlined, WarningOutlined } from '@ant-design/icons'
import { Button } from 'antd'
import { Link } from 'react-router-dom'

import styles from './ErrorPage.module.css'

export type ErrorPageVariant = 403 | 404 | 500

export type ErrorPageProps = {
  variant: ErrorPageVariant
  /** Replaces the default Spanish title for the variant. */
  title?: string
  /** Replaces the default Spanish description for the variant. */
  description?: string
  /** Shown as a secondary "Reintentar" action on the 500 variant only. */
  onRetry?: () => void
}

type VariantCopy = {
  title: string
  description: string
  icon: typeof LockOutlined
}

const COPY: Record<ErrorPageVariant, VariantCopy> = {
  403: {
    title: 'No tienes acceso a esta página',
    description: 'Tu cuenta no tiene permiso para ver este contenido. Pide acceso a un administrador de la organización.',
    icon: LockOutlined,
  },
  404: {
    title: 'No encontramos esta página',
    description: 'La dirección no existe o el contenido se ha movido o eliminado.',
    icon: QuestionCircleOutlined,
  },
  500: {
    title: 'Algo salió mal',
    description: 'No se pudo cargar esta página. Inténtalo de nuevo o vuelve al inicio.',
    icon: WarningOutlined,
  },
}

/** Full-page error for 403, 404 and 500 with a link back home and, for 500, an optional retry. */
export function ErrorPage({ variant, title, description, onRetry }: ErrorPageProps) {
  const copy = COPY[variant]
  const Icon = copy.icon

  return (
    <div className={styles.page} data-variant={variant} role="alert">
      <Icon className={styles.icon} aria-hidden="true" />
      <p className={styles.code}>{variant}</p>
      <h1 className={styles.title}>{title ?? copy.title}</h1>
      <p className={styles.description}>{description ?? copy.description}</p>
      <div className={styles.actions}>
        <Link to="/" className={styles.primary}>
          Volver al inicio
        </Link>
        {variant === 500 && onRetry && (
          <Button onClick={onRetry}>Reintentar</Button>
        )}
      </div>
    </div>
  )
}
