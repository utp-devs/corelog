import { ArrowLeftOutlined } from '@ant-design/icons'
import { Alert, Button, Skeleton, Space, Tabs, Tag, Typography } from 'antd'
import { useEffect, useState } from 'react'
import { Outlet, useLocation, useNavigate, useParams } from 'react-router-dom'

import { ProjectHttpRepository } from '@/adapters/out/http/project-http-repository'
import type { Project, ProjectType, ProjectVisibility } from '@/core/projects/domain/project'
import styles from './ProjectWorkspace.module.css'

// Secciones del proyecto. Cada una es una ruta hija del espacio de trabajo.
const sections = [
  { key: 'board', label: 'Tablero' },
  { key: 'backlog', label: 'Backlog' },
  { key: 'sprints', label: 'Sprints' },
  { key: 'calendar', label: 'Calendario' },
]

const typeLabels: Record<ProjectType, string> = { kanban: 'Kanban', scrum: 'Scrum' }
const visibilityLabels: Record<ProjectVisibility, string> = { private: 'Privado', team: 'Equipo' }

// Cabecera comun de los proyectos: nombre, clave, metadatos y pestañas de navegacion entre secciones.
export function ProjectWorkspace() {
  const { id = '' } = useParams()
  const navigate = useNavigate()
  const location = useLocation()
  const [loaded, setLoaded] = useState<{ id: string; project?: Project; error?: string } | null>(null)

  useEffect(() => {
    let active = true
    new ProjectHttpRepository().get(id)
      .then((project) => { if (active) setLoaded({ id, project }) })
      .catch(() => {
        if (active) setLoaded({ id, error: 'No se pudo abrir el proyecto. Verifica que tengas acceso e intenta de nuevo.' })
      })
    return () => { active = false }
  }, [id])

  const current = loaded?.id === id ? loaded : null
  const activeKey = sections.find((section) => location.pathname.startsWith(`/projects/${id}/${section.key}`))?.key ?? 'board'

  return (
    <div className={styles.workspace}>
      <header className={styles.header}>
        <Button type="link" icon={<ArrowLeftOutlined />} onClick={() => navigate('/projects')} className={styles.back}>
          Proyectos
        </Button>
        {current?.project ? (
          <div className={styles.identity}>
            <div className={styles.titleRow}>
              <Typography.Title level={3} className={styles.title}>{current.project.name}</Typography.Title>
              <Tag className={styles.keyTag}>{current.project.key}</Tag>
            </div>
            <Space size="small" wrap>
              <Tag>{typeLabels[current.project.type]}</Tag>
              <Tag>{visibilityLabels[current.project.visibility]}</Tag>
              {current.project.description && <Typography.Text type="secondary">{current.project.description}</Typography.Text>}
            </Space>
          </div>
        ) : !current?.error ? (
          <Skeleton active paragraph={{ rows: 1 }} />
        ) : null}
      </header>

      {current?.error ? (
        <Alert type="error" showIcon message={current.error} />
      ) : (
        <>
          <Tabs
            activeKey={activeKey}
            onChange={(key) => navigate(`/projects/${id}/${key}`)}
            items={sections.map((section) => ({ key: section.key, label: section.label }))}
            className={styles.tabs}
          />
          <Outlet />
        </>
      )}
    </div>
  )
}
