import type { ReactNode } from 'react'

import { useMediaQuery } from '@/shared/hooks/useMediaQuery'

import styles from './SplitLayout.module.css'

export type SplitSideWidth = 320 | 360 | 400

export type SplitLayoutProps = {
  /** Main content, flexible width. */
  main: ReactNode
  /** Side panel, sticky beside the main content from 992 px. */
  side: ReactNode
  /** Side panel width in px (default 320). Mapped to a fixed class, so no runtime style is needed. */
  sideWidth?: SplitSideWidth
  /** Accessible name of the side panel landmark (default "Panel lateral"). */
  sideLabel?: string
}

/** Main content plus a sticky side panel; the panel stacks below the main content under 992 px. */
export function SplitLayout({ main, side, sideWidth = 320, sideLabel = 'Panel lateral' }: SplitLayoutProps) {
  const stacked = useMediaQuery('(max-width: 991.98px)')

  return (
    <div className={styles.layout} data-stacked={stacked ? 'true' : 'false'} data-side-width={sideWidth}>
      <div className={styles.main}>{main}</div>
      <aside className={styles.side} aria-label={sideLabel}>
        {side}
      </aside>
    </div>
  )
}
