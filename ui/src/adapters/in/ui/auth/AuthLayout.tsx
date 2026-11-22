import { CheckCircleFilled } from '@ant-design/icons'
import { Typography } from 'antd'
import type { ReactNode } from 'react'

import styles from './AuthLayout.module.css'

const highlights = [
  'Incidencias, proyectos y tableros en un solo lugar',
  'Sprints, backlog y calendario por proyecto',
  'Permisos por rol y trazabilidad de cada cambio',
]

type AuthLayoutProps = {
  title: string
  subtitle: string
  footer: ReactNode
  children: ReactNode
}

// Plantilla comun de acceso: panel de marca (solo en escritorio) y formulario centrado.
export function AuthLayout({ title, subtitle, footer, children }: AuthLayoutProps) {
  return (
    <div className={styles.page}>
      <aside className={styles.brandPanel} aria-hidden="true">
        <div className={styles.brandContent}>
          <div className={styles.logoTile}>
            <img src="/brand/corelog-icon-128.png" alt="" width={64} height={64} className={styles.logo} />
          </div>
          <Typography.Title level={2} className={styles.brandTitle}>
            Gestiona tu trabajo con claridad
          </Typography.Title>
          <Typography.Paragraph className={styles.brandText}>
            CoreLog centraliza proyectos, incidencias y planificación para equipos que necesitan control y seguimiento.
          </Typography.Paragraph>
          <ul className={styles.highlights}>
            {highlights.map((item) => (
              <li key={item}>
                <CheckCircleFilled className={styles.highlightIcon} />
                <span>{item}</span>
              </li>
            ))}
          </ul>
        </div>
      </aside>

      <main className={styles.formPanel}>
        <div className={styles.formBox}>
          <img
            src="/brand/corelog-banner-480.png"
            srcSet="/brand/corelog-banner-480.png 480w, /brand/corelog-banner-960.png 960w"
            sizes="180px"
            alt="CoreLog"
            width={180}
            className={styles.mobileBanner}
          />
          <Typography.Title level={3} className={styles.title}>
            {title}
          </Typography.Title>
          <Typography.Paragraph type="secondary" className={styles.subtitle}>
            {subtitle}
          </Typography.Paragraph>
          {children}
          <Typography.Paragraph className={styles.footer}>{footer}</Typography.Paragraph>
        </div>
      </main>
    </div>
  )
}
