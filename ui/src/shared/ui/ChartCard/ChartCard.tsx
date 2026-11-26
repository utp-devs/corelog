import { useId, useState, type ReactNode } from 'react'
import { Button } from 'antd'

import styles from './ChartCard.module.css'

export type ChartTable = {
  headers: string[]
  rows: Array<Array<string | number>>
}

type ChartCardProps = {
  title: string
  description?: string
  /** Gráfico de recharts (el contenedor fija la altura). */
  children: ReactNode
  /** Mismos datos que el gráfico, en tabla accesible. Se oculta visualmente hasta pulsar "Ver datos". */
  table: ChartTable
  isEmpty?: boolean
  emptyText?: string
  'data-testid'?: string
}

/** Tarjeta de gráfico con título, colores de tokens y alternativa en tabla. */
export function ChartCard({
  title,
  description,
  children,
  table,
  isEmpty = false,
  emptyText = 'Sin datos para mostrar',
  'data-testid': testId,
}: ChartCardProps) {
  const [showData, setShowData] = useState(false)
  const titleId = useId()
  const tableId = useId()

  return (
    <section aria-labelledby={titleId} className={styles.card} data-testid={testId}>
      <header className={styles.header}>
        <div>
          <h3 id={titleId} className={styles.title}>
            {title}
          </h3>
          {description && <p className={styles.description}>{description}</p>}
        </div>
        {!isEmpty && (
          <Button size="small" aria-expanded={showData} aria-controls={tableId} onClick={() => setShowData((v) => !v)}>
            Ver datos
          </Button>
        )}
      </header>

      {isEmpty ? (
        <p className={styles.empty}>{emptyText}</p>
      ) : (
        <div className={styles.body}>{children}</div>
      )}

      {!isEmpty && (
        <table
          id={tableId}
          aria-label={`Datos: ${title}`}
          className={showData ? styles.table : `${styles.table} ${styles.srOnly}`}
        >
          <thead>
            <tr>
              {table.headers.map((header) => (
                <th key={header} scope="col">
                  {header}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {table.rows.map((row, rowIndex) => (
              <tr key={rowIndex}>
                {row.map((cell, cellIndex) =>
                  cellIndex === 0 ? (
                    <th key={cellIndex} scope="row">
                      {cell}
                    </th>
                  ) : (
                    <td key={cellIndex}>{cell}</td>
                  ),
                )}
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </section>
  )
}
