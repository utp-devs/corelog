import { Skeleton, Tabs } from 'antd'
import { useCallback, useEffect, useState } from 'react'
import { Outlet, useLocation, useNavigate, useParams } from 'react-router-dom'

import { ProjectHttpRepository } from '@/adapters/out/http/project-http-repository'
import type { Project, ProjectType, ProjectVisibility } from '@/core/projects/domain/project'
import { Breadcrumbs } from '@/shared/ui/Breadcrumbs'
import { ErrorPage } from '@/shared/ui/ErrorPage'
import { ErrorState } from '@/shared/ui/ErrorState'
import { Lozenge } from '@/shared/ui/Lozenge'
import { ProjectAvatar } from '@/shared/ui/ProjectAvatar'
import { useTrackRecent } from '@/shared/ui/CommandPalette/recent'
import { accessVariantOf } from './projectLoadError'
import styles from './ProjectWorkspace.module.css'

// Secciones del proyecto. Cada una es una ruta hija del espacio de trabajo.
const sections = [
  { key: 'board', label: 'Tablero' },
  { key: 'backlog', label: 'Backlog' },
  { key: 'sprints', label: 'Sprints' },
  { key: 'calendar', label: 'Calendario' },
  { key: 'permisos', label: 'Permisos' },
]

const typeLabels: Record<ProjectType, string> = { kanban: 'Kanban', scrum: 'Scrum' }
const visibilityLabels: Record<ProjectVisibility, string> = { private: 'Privado', open: 'Abierto' }

type LoadState =
  | { status: 'loading' }
  | { status: 'ready'; project: Project }
  | { status: 'error'; variant: 403 | 404 | null }

// Cabecera comun de los proyectos: identidad, migas de pan y pestañas de navegacion entre secciones.
export function ProjectWorkspace() {
  const { id = '' } = useParams()
  const navigate = useNavigate()
  const location = useLocation()
  const [attempt, setAttempt] = useState(0)
  const [loaded, setLoaded] = useState<{ key: string; state: LoadState } | null>(null)
  const requestKey = `${id}|${attempt}`

  useEffect(() => {
    let active = true
    new ProjectHttpRepository().get(id)
      .then((project) => { if (active) setLoaded({ key: requestKey, state: { status: 'ready', project } }) })
      .catch((failure: unknown) => {
        if (active) setLoaded({ key: requestKey, state: { status: 'error', variant: accessVariantOf(failure) } })
      })
    return () => { active = false }
    // requestKey encodes id and attempt
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id, attempt])

  const state: LoadState = loaded?.key === requestKey ? loaded.state : { status: 'loading' }
  const project = state.status === 'ready' ? state.project : undefined

  // Recents: the project once its header has loaded.
  useTrackRecent(project
    ? { kind: 'project', id: project.id, title: project.name, subtitle: project.key, link: `/projects/${id}` }
    : null)
  const activeKey = sections.find((section) => location.pathname.startsWith(`/projects/${id}/${section.key}`))?.key ?? 'board'
  const activeLabel = sections.find((section) => section.key === activeKey)?.label ?? 'Tablero'
  const retry = useCallback(() => setAttempt((value) => value + 1), [])

  if (state.status === 'error' && state.variant !== null) {
    return (
      <div className={styles.workspace}>
        <ErrorPage variant={state.variant} />
      </div>
    )
  }

  return (
    <div className={styles.workspace}>
      <Breadcrumbs
        items={[
          { label: 'Proyectos', to: '/projects' },
          { label: project?.key ?? activeLabel },
        ]}
      />

      {state.status === 'loading' && (
        <div className={styles.header} aria-busy="true">
          <Skeleton active avatar={{ size: 40, shape: 'square' }} paragraph={{ rows: 1 }} />
        </div>
      )}

      {state.status === 'error' && (
        <div className={styles.header}>
          <ErrorState message="No se pudo abrir el proyecto. Verifica que tengas acceso e intenta de nuevo." onRetry={retry} />
        </div>
      )}

      {project && (
        <header className={styles.header}>
          <div className={styles.identity}>
            <ProjectAvatar name={project.name} projectKey={project.key} size={40} />
            <div className={styles.titleBlock}>
              <h2 className={styles.title}>{project.name}</h2>
              <div className={styles.meta}>
                <span className={styles.keyTag}>{project.key}</span>
                <Lozenge tone={project.visibility === 'open' ? 'info' : 'warning'}>{visibilityLabels[project.visibility]}</Lozenge>
                <Lozenge>{typeLabels[project.type]}</Lozenge>
              </div>
            </div>
          </div>
          {project.description && <p className={styles.description}>{project.description}</p>}
        </header>
      )}

      {state.status !== 'error' && (
        <>
          <nav aria-label="Secciones del proyecto" className={styles.nav}>
            <Tabs
              activeKey={activeKey}
              onChange={(key) => navigate(`/projects/${id}/${key}`)}
              items={sections.map((section) => ({ key: section.key, label: section.label }))}
              className={styles.tabs}
            />
          </nav>
          <Outlet />
        </>
      )}
    </div>
  )
}
