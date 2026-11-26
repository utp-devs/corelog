import { useId, type ReactNode } from 'react'

import styles from './AuthLayout.module.css'

type AuthLayoutProps = {
  title: string
  subtitle?: string
  /** Helper links under the form (create account, sign in...). Omit or pass null for none. */
  footer?: ReactNode
  children: ReactNode
}

/** Id of the main landmark; the skip link points here. */
export const AUTH_MAIN_ID = 'auth-main'

/**
 * Shared template for the public and onboarding-invite auth screens (S-01, S-02, S-03, S-04).
 * A single centered card (max 440 px) on the token gradient, with the brand banner at its top on every viewport.
 */
export function AuthLayout({ title, subtitle, footer, children }: AuthLayoutProps) {
  const titleId = useId()

  return (
    <div className={styles.page}>
      <a href={`#${AUTH_MAIN_ID}`} className={styles.skipLink}>
        Saltar al formulario
      </a>

      <main id={AUTH_MAIN_ID} tabIndex={-1} className={styles.main}>
        <section className={styles.card} aria-labelledby={titleId}>
          <img
            src="/brand/corelog-banner-480.png"
            srcSet="/brand/corelog-banner-480.png 480w, /brand/corelog-banner-960.png 960w"
            sizes="180px"
            alt="CoreLog"
            width={180}
            height={48}
            className={styles.banner}
          />
          <h1 id={titleId} className={styles.title}>
            {title}
          </h1>
          {subtitle && <p className={styles.subtitle}>{subtitle}</p>}
          {children}
          {footer !== undefined && footer !== null && <p className={styles.footer}>{footer}</p>}
        </section>
      </main>

      <footer className={styles.pageFooter}>CoreLog</footer>
    </div>
  )
}
