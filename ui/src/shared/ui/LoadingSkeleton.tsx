import { Skeleton } from 'antd'

import styles from './LoadingSkeleton.module.css'

type LoadingSkeletonProps = {
  /** Forma del contenido que se está cargando. */
  variant?: 'table' | 'card' | 'detail'
  /** Número de filas en la variante de tabla. */
  rows?: number
}

/** Placeholder de carga con la forma del contenido final. Anuncia el estado de carga a los lectores de pantalla. */
export function LoadingSkeleton({ variant = 'table', rows = 5 }: LoadingSkeletonProps) {
  return (
    <div className={styles.container} role="status" aria-busy="true" aria-label="Cargando">
      {variant === 'table' && <Skeleton active paragraph={{ rows }} title={false} />}
      {variant === 'card' && <Skeleton active paragraph={{ rows: 3 }} />}
      {variant === 'detail' && (
        <>
          <Skeleton active title paragraph={{ rows: 2 }} />
          <Skeleton active paragraph={{ rows: 4 }} />
        </>
      )}
    </div>
  )
}
