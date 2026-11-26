import { Skeleton, Typography } from 'antd'
import { useEffect, useMemo, useState, type ReactNode } from 'react'
import { Link } from 'react-router-dom'

import { DashboardHttpRepository } from '@/adapters/out/http/dashboard-http-repository'
import { ProjectHttpRepository } from '@/adapters/out/http/project-http-repository'
import type { MyDashboard } from '@/core/projects/domain/dashboard'
import type { Project } from '@/core/projects/domain/project'
import { useMediaQuery } from '@/shared/hooks/useMediaQuery'
import { ChartCard } from '@/shared/ui/ChartCard'
import { EmptyState } from '@/shared/ui/EmptyState'
import { ErrorState } from '@/shared/ui/ErrorState'
import { Lozenge } from '@/shared/ui/Lozenge'
import { PageContainer } from '@/shared/ui/PageContainer'
import { PageHeader } from '@/shared/ui/PageHeader'
import { ProgressBar } from '@/shared/ui/ProgressBar'
import { ProjectAvatar } from '@/shared/ui/ProjectAvatar'
import { Section } from '@/shared/ui/Section'
import { StatTile } from '@/shared/ui/StatTile'
import { getRecent, type RecentItem } from '@/shared/ui/CommandPalette/recent'
import styles from './MyDashboardPage.module.css'

const statusLabels: Record<string, string> = {
  open: 'Abiertas',
  in_progress: 'En curso',
  resolved: 'Resueltas',
  closed: 'Cerradas',
}

type DataState = { attempt: number; data?: MyDashboard; error?: string }
type ProjectsState = { attempt: number; items?: Project[]; error: boolean }

/** Panel "Mi resumen": trabajo asignado, proyectos y recientes. Sin trabajo asignado muestra primeros pasos. */
/** Below 768 px the panels stack in one column. */
const MOBILE_QUERY = '(max-width: 767px)'

export function MyDashboardPage() {
  const isMobile = useMediaQuery(MOBILE_QUERY)
  const [attempt, setAttempt] = useState(0)
  const [state, setState] = useState<DataState | null>(null)
  const [projectsState, setProjectsState] = useState<ProjectsState | null>(null)
  // Recientes del navegador: se leen al montar la pagina.
  const [recent] = useState<RecentItem[]>(() => getRecent())

  useEffect(() => {
    let active = true
    new DashboardHttpRepository().me()
      .then((data) => { if (active) setState({ attempt, data }) })
      .catch(() => { if (active) setState({ attempt, error: 'No se pudo cargar tu resumen. Intenta de nuevo.' }) })
    return () => { active = false }
  }, [attempt])

  useEffect(() => {
    let active = true
    new ProjectHttpRepository().list()
      .then((items) => { if (active) setProjectsState({ attempt, items, error: false }) })
      .catch(() => { if (active) setProjectsState({ attempt, error: true }) })
    return () => { active = false }
  }, [attempt])

  const current = state?.attempt === attempt ? state : null
  const data = current?.data
  const projects = projectsState?.attempt === attempt ? projectsState : null
  const retry = () => setAttempt((value) => value + 1)

  const statusRows = useMemo(() => {
    if (!data) return []
    return Object.entries(data.assignedByStatus).map(([status, count]) => ({
      label: statusLabels[status] ?? status,
      count,
    }))
  }, [data])
  const statusTotal = statusRows.reduce((total, row) => total + row.count, 0)

  let body: ReactNode
  if (!current) {
    body = <Skeleton active paragraph={{ rows: 6 }} />
  } else if (current.error) {
    body = <ErrorState message={current.error} onRetry={retry} />
  } else if (data) {
    const hasWork = data.assignedOpen > 0 || data.recentIssues.length > 0
    body = (
      <>
        <div className={styles.stats}>
          <StatTile label="Abiertas asignadas" value={data.assignedOpen} />
          <StatTile label="Vencidas" value={data.overdue} />
          <StatTile label="Proyectos" value={data.projectCount} />
        </div>

        <div className={styles.panels} data-layout={isMobile ? 'stacked' : 'grid'}>
          {!hasWork && (
            <Section title="Primeros pasos" description="Lo que puedes hacer para empezar a trabajar.">
              <ol className={styles.steps}>
                <li>
                  <Link to="/projects">Crea tu primer proyecto</Link>
                  <Typography.Text type="secondary"> para organizar el trabajo en un espacio propio.</Typography.Text>
                </li>
                <li>
                  <Typography.Text>Crea una incidencia con el botón </Typography.Text>
                  <Typography.Text strong>Crear</Typography.Text>
                  <Typography.Text> de la barra superior.</Typography.Text>
                </li>
                <li>
                  <Typography.Text>Abre el tablero de un proyecto para ver y mover tus incidencias.</Typography.Text>
                </li>
              </ol>
            </Section>
          )}

          <ChartCard
            title="Por estado"
            description="Tus incidencias asignadas agrupadas por estado."
            isEmpty={statusRows.length === 0}
            emptyText="Sin trabajo asignado."
            table={{
              headers: ['Estado', 'Incidencias'],
              rows: statusRows.map((row) => [row.label, row.count]),
            }}
          >
            <div className={styles.bars}>
              {statusRows.map((row) => (
                <ProgressBar
                  key={row.label}
                  label={`${row.label}: ${row.count}`}
                  value={statusTotal > 0 ? (row.count / statusTotal) * 100 : 0}
                />
              ))}
            </div>
          </ChartCard>

          <Section title="Lo más urgente">
            {data.recentIssues.length === 0 ? (
              <EmptyState title="No tienes incidencias abiertas" description="Las incidencias que te asignen aparecerán aquí." />
            ) : (
              <ul className={styles.list} aria-label="Incidencias urgentes">
                {data.recentIssues.map((issue) => (
                  <li key={issue.id} className={styles.listItem}>
                    <Link to={`/issues/${issue.id}`} className={styles.itemLink} title={issue.title}>{issue.title}</Link>
                    <Typography.Text type="secondary" className={styles.meta}>
                      {issue.projectKey}{issue.dueDate ? ` · vence ${issue.dueDate}` : ''}
                    </Typography.Text>
                  </li>
                ))}
              </ul>
            )}
          </Section>

          <Section title="Sprints activos">
            {data.activeSprints.length === 0 ? (
              <EmptyState title="No hay sprints activos" description="Cuando un sprint de tus proyectos esté activo, aparecerá aquí." />
            ) : (
              <ul className={styles.list} aria-label="Sprints activos">
                {data.activeSprints.map((sprint) => (
                  <li key={sprint.id} className={styles.listItem}>
                    <span>{sprint.projectKey} · {sprint.name}{sprint.endDate ? ` (fin ${sprint.endDate})` : ''}</span>
                    <Lozenge tone="success">Activo</Lozenge>
                  </li>
                ))}
              </ul>
            )}
          </Section>

          <Section title="Mis proyectos">
            {!projects ? (
              <Skeleton active paragraph={{ rows: 3 }} />
            ) : projects.error ? (
              <Typography.Text type="secondary">No se pudieron cargar tus proyectos.</Typography.Text>
            ) : !projects.items || projects.items.length === 0 ? (
              <EmptyState
                title="Aún no tienes proyectos"
                description="Crea un proyecto para empezar a organizar el trabajo."
                action={<Link to="/projects">Ir a proyectos</Link>}
              />
            ) : (
              <ul className={styles.list} aria-label="Mis proyectos">
                {projects.items.map((project) => (
                  <li key={project.id} className={styles.projectItem}>
                    <ProjectAvatar name={project.name} projectKey={project.key} size={24} />
                    <Link to={`/projects/${project.id}/board`} className={styles.itemLink} title={project.name}>{project.name}</Link>
                    <Typography.Text type="secondary" className={styles.meta}>{project.key}</Typography.Text>
                  </li>
                ))}
              </ul>
            )}
          </Section>

          <Section title="Visto recientemente">
            {recent.length === 0 ? (
              <EmptyState title="Aún no has abierto nada" description="Los proyectos e incidencias que abras aparecerán aquí." />
            ) : (
              <ul className={styles.list} aria-label="Vistos recientemente">
                {recent.map((item) => (
                  <li key={`${item.kind}-${item.id}`} className={styles.listItem}>
                    <Link to={item.link} className={styles.itemLink} title={item.title}>{item.title}</Link>
                    <Lozenge tone="neutral">{item.kind === 'project' ? 'Proyecto' : 'Incidencia'}</Lozenge>
                  </li>
                ))}
              </ul>
            )}
          </Section>
        </div>
      </>
    )
  }

  return (
    <PageContainer variant="wide" pageName="dashboard" ready={!!data}>
      <PageHeader title="Mi resumen" description="Tu trabajo asignado, lo que vence y los sprints activos de tus proyectos." />
      {body}
    </PageContainer>
  )
}
