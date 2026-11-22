import { ExclamationCircleOutlined } from '@ant-design/icons'
import { Button } from 'antd'

import styles from './ErrorState.module.css'

type ErrorStateProps = {
  title?: string
  message: string
  onRetry?: () => void
}

/** Error de carga con acción de reintento. Usa el rol alert para que los lectores de pantalla lo anuncien. */
export function ErrorState({ title = 'No se pudo cargar la información', message, onRetry }: ErrorStateProps) {
  return (
    <div className={styles.container} role="alert">
      <ExclamationCircleOutlined className={styles.icon} aria-hidden="true" />
      <p className={styles.title}>{title}</p>
      <p className={styles.message}>{message}</p>
      {onRetry && (
        <Button type="primary" onClick={onRetry}>
          Reintentar
        </Button>
      )}
    </div>
  )
}
