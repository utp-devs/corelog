import { CheckCircleFilled } from '@ant-design/icons'
import { useId, type ReactNode } from 'react'

import styles from './AuthLayout.module.css'

const highlights = [
  'Incidencias, proyectos y tableros en un solo lugar',
  'Sprints, backlog y calendario por proyecto',
  'Permisos por rol y trazabilidad de cada cambio',
]

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
 * Shared template for the public and onboarding auth screens (S-01, S-02, S-04, S-05).
 * Desktop: brand panel on the left and a centered card. Mobile: the card alone with the banner on top.
 */
export function AuthLayout({ title, subtitle, footer, children }: AuthLayoutProps) {
  const titleId = useId()

  return (
    <div className={styles.page}>
      <a href={`#${AUTH_MAIN_ID}`} className={styles.skipLink}>
        Saltar al formulario
      </a>

      <aside className={styles.brandPanel} aria-label="CoreLog">
        <div className={styles.brandContent}>
          <div className={styles.logoTile}>
            <img src="/brand/corelog-icon-128.png" alt="" width={64} height={64} className={styles.logo} />
          </div>
          <p className={styles.brandTitle}>Gestiona tu trabajo con claridad</p>
          <p className={styles.brandText}>
            CoreLog centraliza proyectos, incidencias y planificación para equipos que necesitan control y seguimiento.
          </p>
          <ul className={styles.highlights}>
            {highlights.map((item) => (
              <li key={item}>
                <CheckCircleFilled className={styles.highlightIcon} aria-hidden="true" />
                <span>{item}</span>
              </li>
            ))}
          </ul>
        </div>
      </aside>

      <main id={AUTH_MAIN_ID} tabIndex={-1} className={styles.formPanel}>
        <section className={styles.card} aria-labelledby={titleId}>
          <img
            src="/brand/corelog-banner-480.png"
            srcSet="/brand/corelog-banner-480.png 480w, /brand/corelog-banner-960.png 960w"
            sizes="180px"
            alt="CoreLog"
            width={180}
            height={48}
            className={styles.mobileBanner}
          />
          <h1 id={titleId} className={styles.title}>
            {title}
          </h1>
          {subtitle && <p className={styles.subtitle}>{subtitle}</p>}
          {children}
          {footer !== undefined && footer !== null && <p className={styles.footer}>{footer}</p>}
        </section>
      </main>
    </div>
  )
}
