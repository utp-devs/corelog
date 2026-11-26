import type { ReactNode } from 'react'

import { useMediaQuery } from '@/shared/hooks/useMediaQuery'

import styles from './PageContainer.module.css'

export type PageVariant = 'reading' | 'wide' | 'fluid'

export type PageContainerProps = {
  /** reading: forms and detail (max --cl-content-max). wide: data pages (max --cl-content-wide). fluid: edge to edge. */
  variant: PageVariant
  /** Right panel (320 px). Moves below the content under 992 px. */
  sidebar?: ReactNode
  /** Sets data-ready="true" when true, "false" otherwise. Lets audits and e2e wait for real content. */
  ready?: boolean
  /** Sets data-page for layout audits and e2e selectors. */
  pageName?: string
  /**
   * Fill the viewport below the top bar: the content column grows to at least
   * calc(100dvh - --cl-topbar-h - 2 * --cl-gutter), so a child with flex-grow (Section fill, DataTable fill)
   * extends to the bottom instead of leaving an empty band. Default false.
   */
  fill?: boolean
  children: ReactNode
}

/**
 * Wraps the content of a page: max-width per template, gutters, bottom padding and an optional
 * right panel. The shell owns the <main> landmark, so this renders a div.
 */
export function PageContainer({
  variant,
  sidebar,
  ready,
  pageName,
  fill = false,
  children,
}: PageContainerProps) {
  const stacked = useMediaQuery('(max-width: 991.98px)')
  const hasSidebar = sidebar !== undefined && sidebar !== null && sidebar !== false

  return (
    <div
      className={`${styles.root} ${styles[variant]}`}
      data-variant={variant}
      data-ready={ready ? 'true' : 'false'}
      data-page={pageName}
      data-fill={fill ? 'true' : 'false'}
      data-has-sidebar={hasSidebar ? 'true' : 'false'}
      data-stacked={hasSidebar && stacked ? 'true' : 'false'}
    >
      <div className={styles.content}>{children}</div>
      {hasSidebar && (
        <aside className={styles.sidebar} aria-label="Panel lateral">
          {sidebar}
        </aside>
      )}
    </div>
  )
}
