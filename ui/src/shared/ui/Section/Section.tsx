import { useId, type ReactNode } from 'react'

import styles from './Section.module.css'

export type SectionHeadingLevel = 2 | 3 | 4

export type SectionProps = {
  title?: ReactNode
  description?: ReactNode
  /** Buttons or links aligned to the right of the header. */
  actions?: ReactNode
  /** Footer row below the body (totals, pagination, form submit). */
  footer?: ReactNode
  /** Body padding (default true). */
  padded?: boolean
  /** No body padding at all, for tables and lists that draw their own edges. */
  flush?: boolean
  /** Grows to fill the remaining height of a flex column (PageContainer fill); the body stretches with it. Default false. */
  fill?: boolean
  /** Heading level of the title (default 2). */
  headingLevel?: SectionHeadingLevel
  /** Accessible name when there is no visible title. */
  ariaLabel?: string
  children?: ReactNode
}

/** Titled card block: header (title, description, actions), body and optional footer. */
export function Section({
  title,
  description,
  actions,
  footer,
  padded = true,
  flush = false,
  fill = false,
  headingLevel = 2,
  ariaLabel,
  children,
}: SectionProps) {
  const titleId = useId()
  const hasHeader = title !== undefined || description !== undefined || actions !== undefined
  const Heading = `h${headingLevel}` as 'h2' | 'h3' | 'h4'
  const bodyPadded = padded && !flush

  return (
    <section
      className={styles.section}
      data-fill={fill ? 'true' : 'false'}
      aria-labelledby={title !== undefined ? titleId : undefined}
      aria-label={title === undefined ? ariaLabel : undefined}
    >
      {hasHeader && (
        <header className={styles.header}>
          <div className={styles.titleBlock}>
            {title !== undefined && (
              <Heading id={titleId} className={styles.title}>
                {title}
              </Heading>
            )}
            {description !== undefined && <p className={styles.description}>{description}</p>}
          </div>
          {actions !== undefined && <div className={styles.actions}>{actions}</div>}
        </header>
      )}
      <div
        className={[bodyPadded ? styles.body : styles.bodyFlush, fill ? styles.grow : ''].filter(Boolean).join(' ')}
      >
        {children}
      </div>
      {footer !== undefined && <footer className={styles.footer}>{footer}</footer>}
    </section>
  )
}
