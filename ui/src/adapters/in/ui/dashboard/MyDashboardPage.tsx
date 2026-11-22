import { Alert, Card, Empty, Skeleton, Space, Statistic, Tag, Typography } from 'antd'
import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'

import { DashboardHttpRepository } from '@/adapters/out/http/dashboard-http-repository'
import type { MyDashboard } from '@/core/projects/domain/dashboard'
import { PageHeader } from '@/shared/ui/PageHeader'
import styles from './MyDashboardPage.module.css'

const statusLabels: Record<string, string> = {
  open: 'Abiertas',
  in_progress: 'En curso',
  resolved: 'Resueltas',
  closed: 'Cerradas',
}

export function MyDashboardPage() {
  const [state, setState] = useState<{ data?: MyDashboard; error?: string } | null>(null)

  useEffect(() => {
    let active = true
    new DashboardHttpRepository().me()
      .then((data) => { if (active) setState({ data }) })
      .catch(() => { if (active) setState({ error: 'No se pudo cargar tu resumen. Intenta de nuevo.' }) })
    return () => { active = false }
  }, [])

  return (
    <div>
      <PageHeader title="Mi resumen" description="Tu trabajo asignado, lo que vence y los sprints activos de tus proyectos." />
      {!state ? <Skeleton active paragraph={{ rows: 6 }} /> : state.error ? (
        <Alert type="error" showIcon message={state.error} />
      ) : state.data && (
        <Space orientation="vertical" size="large" className={styles.stack}>
          <div className={styles.stats}>
            <Card><Statistic title="Abiertas asignadas" value={state.data.assignedOpen} /></Card>
            <Card><Statistic title="Vencidas" value={state.data.overdue} /></Card>
            <Card><Statistic title="Proyectos" value={state.data.projectCount} /></Card>
          </div>

          <Card title="Por estado">
            <Space wrap>
              {Object.entries(state.data.assignedByStatus).map(([status, count]) => (
                <Tag key={status}>{statusLabels[status] ?? status}: {count}</Tag>
              ))}
              {Object.keys(state.data.assignedByStatus).length === 0 && <Typography.Text type="secondary">Sin trabajo asignado.</Typography.Text>}
            </Space>
          </Card>

          <Card title="Sprints activos">
            {state.data.activeSprints.length === 0 ? <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="No hay sprints activos en tus proyectos." /> : (
              <ul className={styles.list}>
                {state.data.activeSprints.map((sprint) => (
                  <li key={sprint.id}>
                    {sprint.projectKey} · {sprint.name}{sprint.endDate ? ` (fin ${sprint.endDate})` : ''}
                  </li>
                ))}
              </ul>
            )}
          </Card>

          <Card title="Lo más urgente">
            {state.data.recentIssues.length === 0 ? <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="No tienes incidencias abiertas." /> : (
              <ul className={styles.list}>
                {state.data.recentIssues.map((issue) => (
                  <li key={issue.id}>
                    <Link to={`/issues/${issue.id}`}>{issue.title}</Link>
                    <Typography.Text type="secondary"> · {issue.projectKey}{issue.dueDate ? ` · vence ${issue.dueDate}` : ''}</Typography.Text>
                  </li>
                ))}
              </ul>
            )}
          </Card>
        </Space>
      )}
    </div>
  )
}
