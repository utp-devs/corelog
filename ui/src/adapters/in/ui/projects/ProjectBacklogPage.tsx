import { ArrowDownOutlined, ArrowUpOutlined } from '@ant-design/icons'
import { Alert, Button, Typography } from 'antd'
import { useCallback, useEffect, useMemo, useState } from 'react'
import { Link, useParams } from 'react-router-dom'

import { BacklogHttpRepository } from '@/adapters/out/http/backlog-http-repository'
import { ProjectHttpRepository } from '@/adapters/out/http/project-http-repository'
import type { BacklogItem } from '@/core/projects/domain/backlog'
import type { Project } from '@/core/projects/domain/project'
import { httpStatusOf } from '@/core/shared/app-error'
import type { TicketPriority, TicketStatus } from '@/core/tickets/domain/ticket'
import { DataTable, type DataTableColumn } from '@/shared/ui/DataTable'
import { EmptyState } from '@/shared/ui/EmptyState'
import { ErrorPage } from '@/shared/ui/ErrorPage'
import { Lozenge, type LozengeCategory } from '@/shared/ui/Lozenge'
import { PageContainer } from '@/shared/ui/PageContainer'
import { PageHeader } from '@/shared/ui/PageHeader'
import { PriorityTag } from '@/shared/ui/PriorityTag'
import { Section } from '@/shared/ui/Section'
import { accessVariantOf, type AccessVariant } from './projectLoadError'
import styles from './ProjectBacklogPage.module.css'

const statusLabels: Record<TicketStatus, string> = {
  open: 'Abierta',
  in_progress: 'En curso',
  resolved: 'Resuelta',
  closed: 'Cerrada',
}

const statusCategories: Record<TicketStatus, LozengeCategory> = {
  open: 'todo',
  in_progress: 'progress',
  resolved: 'done',
  closed: 'done',
}

// Mensajes en español para reordenar el backlog.
function backlogErrorMessage(error: unknown): string {
  const status = httpStatusOf(error)
  if (status === 403) return 'No tienes permiso para ordenar el backlog de este proyecto.'
  if (status === 422) return 'La posición de la incidencia no es válida.'
  return 'No se pudo reordenar el backlog. Intenta de nuevo.'
}

type LoadResult = {
  projectId: string
  project?: Project
  items?: BacklogItem[]
  error?: { variant: AccessVariant | null; message: string }
}

export function ProjectBacklogPage() {
  const { id = '' } = useParams()
  const [result, setResult] = useState<LoadResult | null>(null)
  const [attempt, setAttempt] = useState(0)
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
      .catch((failure: unknown) => {
        if (active) setResult({
          projectId: id,
          error: {
            variant: accessVariantOf(failure),
            message: 'No se pudo cargar el backlog. Verifica que tengas acceso al proyecto e intenta de nuevo.',
          },
        })
      })
    return () => { active = false }
  }, [id, attempt])

  const current = result?.projectId === id ? result : null
  const items = useMemo(() => current?.items ?? [], [current])

  // Mueve la incidencia de la posicion index a target. La API devuelve el backlog ya renumerado.
  const move = useCallback((index: number, target: number) => {
    const item = items[index]
    if (!item || target < 0 || target >= items.length) return
    setMoveError(undefined)
    new BacklogHttpRepository()
      .move(id, item.id, target)
      .then((updated) => setResult((previous) => (previous && previous.projectId === id ? { ...previous, items: updated } : previous)))
      .catch((error: unknown) => setMoveError(backlogErrorMessage(error)))
  }, [id, items])

  if (current?.error?.variant) {
    return (
      <PageContainer variant="wide" pageName="backlog">
        <ErrorPage variant={current.error.variant} />
      </PageContainer>
    )
  }

  const columns: DataTableColumn<BacklogItem>[] = [
    {
      key: 'position',
      title: '#',
      width: 56,
      render: (_value, _item, index) => <span className={styles.position}>{index + 1}</span>,
    },
    {
      key: 'summary',
      title: 'Incidencia',
      mobilePrimary: true,
      render: (_value, item) => (
        <div className={styles.main}>
          <Link to={`/issues/${item.id}`} className={styles.summary} title={item.summary}>{item.summary}</Link>
          <Typography.Text type="secondary">{item.type}</Typography.Text>
        </div>
      ),
    },
    {
      key: 'priority',
      title: 'Prioridad',
      render: (_value, item) => <PriorityTag priority={item.priority as TicketPriority} />,
    },
    {
      key: 'status',
      title: 'Estado',
      render: (_value, item) => (
        <Lozenge category={statusCategories[item.status] ?? 'todo'}>{statusLabels[item.status] ?? item.status}</Lozenge>
      ),
    },
    {
      key: 'points',
      title: 'Puntos',
      align: 'right',
      render: (_value, item) => (item.storyPoints !== undefined ? String(item.storyPoints) : '-'),
    },
    {
      key: 'order',
      title: 'Orden',
      align: 'right',
      render: (_value, item) => {
        const index = items.indexOf(item)
        return (
          <div className={styles.moveButtons}>
            <Button
              size="small"
              icon={<ArrowUpOutlined aria-hidden="true" />}
              aria-label={`Subir ${item.summary}`}
              disabled={index <= 0}
              onClick={() => move(index, index - 1)}
            />
            <Button
              size="small"
              icon={<ArrowDownOutlined aria-hidden="true" />}
              aria-label={`Bajar ${item.summary}`}
              disabled={index < 0 || index === items.length - 1}
              onClick={() => move(index, index + 1)}
            />
          </div>
        )
      },
    },
  ]

  return (
    <PageContainer variant="wide" pageName="backlog" ready={current !== null && !current.error} fill>
      <PageHeader title="Backlog" description="Las incidencias de arriba se trabajan primero. Usa las flechas para reordenarlas." />
      {moveError && (
        <Alert type="warning" showIcon closable message={moveError} onClose={() => setMoveError(undefined)} />
      )}
      <Section flush fill>
        <DataTable<BacklogItem>
          tableKey="project-backlog"
          columns={columns}
          data={items}
          fill
          rowKey="id"
          loading={current === null}
          error={current?.error?.message ?? null}
          onRetry={() => setAttempt((value) => value + 1)}
          emptyState={
            <EmptyState
              title="El backlog está vacío"
              description="Crea incidencias o impórtalas para empezar a planificar."
            />
          }
          mobileRender={(item) => {
            const index = items.indexOf(item)
            return (
              <div className={styles.mobileCard}>
                <Link to={`/issues/${item.id}`} className={styles.summary} title={item.summary}>{item.summary}</Link>
                <div className={styles.mobileMeta}>
                  <PriorityTag priority={item.priority as TicketPriority} />
                  <Lozenge category={statusCategories[item.status] ?? 'todo'}>{statusLabels[item.status] ?? item.status}</Lozenge>
                </div>
                <div className={styles.moveButtons}>
                  <Button size="small" icon={<ArrowUpOutlined aria-hidden="true" />} aria-label={`Subir ${item.summary}`} disabled={index <= 0} onClick={() => move(index, index - 1)} />
                  <Button size="small" icon={<ArrowDownOutlined aria-hidden="true" />} aria-label={`Bajar ${item.summary}`} disabled={index < 0 || index === items.length - 1} onClick={() => move(index, index + 1)} />
                </div>
              </div>
            )
          }}
        />
      </Section>
    </PageContainer>
  )
}
