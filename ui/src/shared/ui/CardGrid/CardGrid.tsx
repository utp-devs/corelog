import type { CSSProperties, ReactNode } from 'react'

import styles from './CardGrid.module.css'

export type CardGridProps = {
  /** Minimum card width in px; columns fill the row with repeat(auto-fill, minmax(min, 1fr)). Default 280. */
  minCardWidth?: number
  /** Accessible name of the grid group; set it when the cards have no heading of their own. */
  ariaLabel?: string
  /**
   * Frames the grid in a surface panel that grows to the bottom of a filled page (PageContainer fill),
   * so a short list leaves no bare band under its cards. Default false.
   */
  fill?: boolean
  children: ReactNode
}

/**
 * Grid of equal-height cards (docs/11-testing-strategy.md section 5, check 5 and 7). Items stretch to
 * the tallest card in their row and the last row keeps its cards on the left (no orphan gaps).
 */
export function CardGrid({ minCardWidth = 280, ariaLabel, fill = false, children }: CardGridProps) {
  const grid = (
    <div
      className={styles.grid}
      data-card-grid=""
      role={ariaLabel && !fill ? 'group' : undefined}
      aria-label={ariaLabel && !fill ? ariaLabel : undefined}
      /* runtime-style */
      style={{ '--cl-card-min': `${minCardWidth}px` } as CSSProperties}
    >
      {children}
    </div>
  )
  if (!fill) return grid
  return (
    <div className={styles.panel} data-fill="true" role={ariaLabel ? 'group' : undefined} aria-label={ariaLabel}>
      {grid}
    </div>
  )
}
