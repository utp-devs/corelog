import { Typography } from 'antd'
import type { ReactNode } from 'react'

import styles from './PageHeader.module.css'

type PageHeaderProps = {
  title: string
  description?: string
  actions?: ReactNode
}

export function PageHeader({ title, description, actions }: PageHeaderProps) {
  return (
    <div className={styles.header}>
      <div>
        <Typography.Title level={2} className={styles.title}>
          {title}
        </Typography.Title>
        {description && (
          <Typography.Text type="secondary" className={styles.description}>
            {description}
          </Typography.Text>
        )}
      </div>
      {actions && <div>{actions}</div>}
    </div>
  )
}
