import { Alert, Button, Empty, Skeleton, Space, Tag, Typography } from 'antd'
import { ArrowDownOutlined, ArrowUpOutlined } from '@ant-design/icons'
import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'

import { BacklogHttpRepository } from '@/adapters/out/http/backlog-http-repository'
import { ProjectHttpRepository } from '@/adapters/out/http/project-http-repository'
import type { BacklogItem } from '@/core/projects/domain/backlog'
import type { Project } from '@/core/projects/domain/project'
import type { TicketPriority, TicketStatus } from '@/core/tickets/domain/ticket'
import { PageHeader } from '@/shared/ui/PageHeader'
import { PriorityTag } from '@/shared/ui/PriorityTag'
import styles from './ProjectBacklogPage.module.css'

const statusLabels: Record<TicketStatus, string> = {
  open: 'Abierta',
  in_progress: 'En curso',
  resolved: 'Resuelta',
  closed: 'Cerrada',
}

// Mensajes en español para reordenar el backlog.
function backlogErrorMessage(error: unknown): string {
  const status = (error as { response?: { status?: number } }).response?.status
  if (status === 403) return 'No tienes permiso para ordenar el backlog de este proyecto.'
  if (status === 422) return 'La posición de la incidencia no es válida.'
  return 'No se pudo reordenar el backlog. Intenta de nuevo.'
}

export function ProjectBacklogPage() {
  const { id = '' } = useParams()
  const [result, setResult] = useState<{
    projectId: string
    project?: Project
    items?: BacklogItem[]
    error?: string
  } | null>(null)
  const [moveError, setMoveError] = useState<string | undefined>(undefined)

  useEffect(() => {
    let active = true
    const projects = new ProjectHttpRepository()
    const backlog = new BacklogHttpRepository()
    void Promise.all([projects.get(id), backlog.list(id)])
      .then(([loadedProject, loadedItems]) => {
        if (!active) return
        setResult({ projectId: id, project: loadedProject, items: loadedItems })
      })
      .catch(() => {
        if (active) setResult({
          projectId: id,
          error: 'No se pudo cargar el backlog. Verifica que tengas acceso al proyecto e intenta de nuevo.',
        })
      })
    return () => { active = false }
  }, [id])

  const current = result?.projectId === id ? result : null
  const items = current?.items ?? []

  // Mueve la incidencia de la posicion index a target. La API devuelve el backlog ya renumerado.
  const move = (index: number, target: number) => {
    const item = items[index]
    if (!item || target < 0 || target >= items.length) return
    setMoveError(undefined)
    new BacklogHttpRepository()
      .move(id, item.id, target)
      .then((updated) => setResult((previous) => (previous && previous.projectId === id ? { ...previous, items: updated } : previous)))
      .catch((error: unknown) => setMoveError(backlogErrorMessage(error)))
  }

  return (
    <div>
      <PageHeader title="Backlog" description="Las incidencias de arriba se trabajan primero. Usa las flechas para reordenarlas." />
      {moveError && (
        <Alert type="warning" showIcon closable message={moveError} onClose={() => setMoveError(undefined)} className={styles.alert} />
      )}
      {!current ? <Skeleton active paragraph={{ rows: 5 }} /> : current.error ? <Alert type="error" showIcon message={current.error} /> : items.length === 0 ? (
        <Empty description="El proyecto todavía no tiene incidencias en el backlog." />
      ) : (
        <ol className={styles.list} aria-label="Backlog ordenado">
          {items.map((item, index) => (
            <li key={item.id} className={styles.row}>
              <span className={styles.position} aria-hidden="true">{index + 1}</span>
              <div className={styles.main}>
                <Link to={`/issues/${item.id}`} className={styles.summary}>{item.summary}</Link>
                <Space wrap size={[6, 6]}>
                  <PriorityTag priority={item.priority as TicketPriority} />
                  <Tag>{statusLabels[item.status] ?? item.status}</Tag>
                  {item.storyPoints !== undefined && <Tag>{item.storyPoints} puntos</Tag>}
                  <Typography.Text type="secondary">{item.type}</Typography.Text>
                </Space>
              </div>
              <Space size="small">
                <Button
                  icon={<ArrowUpOutlined />}
                  aria-label={`Subir ${item.summary}`}
                  disabled={index === 0}
                  onClick={() => move(index, index - 1)}
                />
                <Button
                  icon={<ArrowDownOutlined />}
                  aria-label={`Bajar ${item.summary}`}
                  disabled={index === items.length - 1}
                  onClick={() => move(index, index + 1)}
                />
              </Space>
            </li>
          ))}
        </ol>
      )}
    </div>
  )
}
