import { Alert, Button, Input, Radio, Select, Skeleton, Space } from 'antd'
import { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'

import { BacklogHttpRepository } from '@/adapters/out/http/backlog-http-repository'
import { SprintHttpRepository } from '@/adapters/out/http/sprint-http-repository'
import type { BacklogItem } from '@/core/projects/domain/backlog'
import type { BurndownPoint, Sprint, SprintIssue, SprintStatus, VelocityPoint } from '@/core/projects/domain/sprint'
import { httpStatusOf } from '@/core/shared/app-error'
import { ChartCard } from '@/shared/ui/ChartCard'
import { DataTable, type DataTableColumn } from '@/shared/ui/DataTable'
import { EmptyState } from '@/shared/ui/EmptyState'
import { ErrorPage } from '@/shared/ui/ErrorPage'
import { ErrorState } from '@/shared/ui/ErrorState'
import { Lozenge, type LozengeTone } from '@/shared/ui/Lozenge'
import { PageContainer } from '@/shared/ui/PageContainer'
import { PageHeader } from '@/shared/ui/PageHeader'
import { Section } from '@/shared/ui/Section'
import { StatTile } from '@/shared/ui/StatTile'
import { BurndownChart } from './BurndownChart'
import { accessVariantOf, type AccessVariant } from './projectLoadError'
import styles from './ProjectSprintsPage.module.css'

const statusLabels: Record<SprintStatus, string> = { planned: 'Planificado', active: 'Activo', closed: 'Cerrado' }
const statusTones: Record<SprintStatus, LozengeTone> = { planned: 'info', active: 'success', closed: 'neutral' }
const issueStatusLabels: Record<string, string> = { open: 'Abierta', in_progress: 'En curso', resolved: 'Resuelta', closed: 'Cerrada' }

// Mensajes en español para las acciones de sprint, segun el codigo de la API.
function sprintErrorMessage(error: unknown): string {
  const status = httpStatusOf(error)
  if (status === 403) return 'Solo el lead del proyecto o el administrador pueden gestionar sprints.'
  if (status === 409) return 'Esa acción no está permitida en el estado actual del sprint o ya hay otro sprint activo.'
  if (status === 422) return 'Revisa los datos del sprint: fechas en formato AAAA-MM-DD y fin posterior al inicio.'
  return 'No se pudo completar la acción. Intenta de nuevo.'
}

type LoadError = { variant: AccessVariant | null; message: string }

export function ProjectSprintsPage() {
  const { id = '' } = useParams()
  const [sprints, setSprints] = useState<Sprint[] | undefined>(undefined)
  const [velocity, setVelocity] = useState<VelocityPoint[]>([])
  const [loadError, setLoadError] = useState<LoadError | undefined>(undefined)
  const [attempt, setAttempt] = useState(0)
  const [selectedId, setSelectedId] = useState<string | undefined>(undefined)
  const [issues, setIssues] = useState<SprintIssue[]>([])
  const [burndown, setBurndown] = useState<BurndownPoint[]>([])
  const [backlog, setBacklog] = useState<BacklogItem[]>([])
  const [actionError, setActionError] = useState<string | undefined>(undefined)
  const [name, setName] = useState('')
  const [goal, setGoal] = useState('')
  const [startDate, setStartDate] = useState('')
  const [endDate, setEndDate] = useState('')
  const [closeMode, setCloseMode] = useState<'backlog' | 'sprint'>('backlog')
  const [closeTarget, setCloseTarget] = useState<string | undefined>(undefined)
  const [issueToAdd, setIssueToAdd] = useState<string | undefined>(undefined)

  const sprintRepo = new SprintHttpRepository()

  // Carga el proyecto, sus sprints y la velocidad. Se vuelve a llamar tras cada accion.
  const reload = async (preferId?: string) => {
    const [loadedSprints, loadedVelocity] = await Promise.all([sprintRepo.list(id), sprintRepo.velocity(id)])
    setSprints(loadedSprints)
    setVelocity(loadedVelocity)
    const current = preferId ?? selectedId
    const next = loadedSprints.find((sprint) => sprint.id === current)
      ?? loadedSprints.find((sprint) => sprint.status === 'active')
      ?? loadedSprints.find((sprint) => sprint.status === 'planned')
      ?? loadedSprints[0]
    setSelectedId(next?.id)
  }

  useEffect(() => {
    let active = true
    void Promise.all([sprintRepo.list(id), sprintRepo.velocity(id)])
      .then(([loadedSprints, loadedVelocity]) => {
        if (!active) return
        setLoadError(undefined)
        setSprints(loadedSprints)
        setVelocity(loadedVelocity)
        const first = loadedSprints.find((sprint) => sprint.status === 'active') ?? loadedSprints.find((sprint) => sprint.status === 'planned') ?? loadedSprints[0]
        setSelectedId(first?.id)
      })
      .catch((failure: unknown) => {
        if (active) setLoadError({
          variant: accessVariantOf(failure),
          message: 'No se pudieron cargar los sprints. Verifica que tengas acceso al proyecto e intenta de nuevo.',
        })
      })
    return () => { active = false }
    // eslint-disable-next-line react-hooks/exhaustive-deps -- la carga inicial depende solo del proyecto y del reintento
  }, [id, attempt])

  // Al cambiar de sprint cargamos sus incidencias, el burndown y el backlog para asignar.
  useEffect(() => {
    if (!selectedId) return
    let active = true
    void Promise.all([
      sprintRepo.listIssues(id, selectedId),
      sprintRepo.burndown(id, selectedId).catch(() => [] as BurndownPoint[]),
      new BacklogHttpRepository().list(id),
    ]).then(([loadedIssues, loadedBurndown, loadedBacklog]) => {
      if (!active) return
      setIssues(loadedIssues)
      setBurndown(loadedBurndown)
      setBacklog(loadedBacklog)
    }).catch(() => {
      if (active) setActionError('No se pudieron cargar las incidencias del sprint.')
    })
    return () => { active = false }
    // eslint-disable-next-line react-hooks/exhaustive-deps -- sprintRepo es estable y se recrea solo por render
  }, [id, selectedId])

  const selected = sprints?.find((sprint) => sprint.id === selectedId)
  const plannedTargets = (sprints ?? []).filter((sprint) => sprint.status === 'planned')
  const issueIdsInSprint = new Set(issues.map((issue) => issue.id))
  const assignable = backlog.filter((item) => !issueIdsInSprint.has(item.id))

  // Ejecuta una accion de sprint: muestra el error en espanol o recarga la lista.
  const run = (action: () => Promise<unknown>, preferId?: string) => {
    setActionError(undefined)
    action()
      .then(() => reload(preferId))
      .catch((error: unknown) => setActionError(sprintErrorMessage(error)))
  }

  const create = () => {
    if (!name.trim()) {
      setActionError('El nombre del sprint es obligatorio.')
      return
    }
    setActionError(undefined)
    sprintRepo
      .create(id, { name, goal, startDate, endDate })
      .then((created) => {
        setName('')
        setGoal('')
        setStartDate('')
        setEndDate('')
        return reload(created.id)
      })
      .catch((error: unknown) => setActionError(sprintErrorMessage(error)))
  }

  if (loadError?.variant) {
    return (
      <PageContainer variant="wide" pageName="sprints">
        <ErrorPage variant={loadError.variant} />
      </PageContainer>
    )
  }

  const issueColumns: DataTableColumn<SprintIssue>[] = [
    { key: 'title', title: 'Incidencia', mobilePrimary: true, render: (_value, issue) => <span className={styles.issueTitle} title={issue.title}>{issue.title}</span> },
    {
      key: 'status',
      title: 'Estado',
      render: (_value, issue) => <Lozenge tone="neutral">{issueStatusLabels[issue.status] ?? issue.status}</Lozenge>,
    },
    {
      key: 'points',
      title: 'Puntos',
      align: 'right',
      render: (_value, issue) => (issue.storyPoints !== undefined ? String(issue.storyPoints) : '-'),
    },
    {
      key: 'actions',
      title: '',
      align: 'right',
      render: (_value, issue) => selected && selected.status !== 'closed' ? (
        <Button size="small" aria-label={`Quitar ${issue.title}`} onClick={() => run(() => sprintRepo.removeIssue(id, selected.id, issue.id), selected.id)}>
          Quitar
        </Button>
      ) : null,
    },
  ]

  return (
    <PageContainer variant="wide" pageName="sprints" ready={sprints !== undefined}>
      <PageHeader title="Sprints" description="Planifica el trabajo, activa un sprint y ciérralo al terminar." />
      {loadError && <ErrorState message={loadError.message} onRetry={() => setAttempt((value) => value + 1)} />}
      {actionError && <Alert type="warning" showIcon closable message={actionError} onClose={() => setActionError(undefined)} />}

      {!loadError && !sprints && <Skeleton active paragraph={{ rows: 6 }} />}

      {sprints && !loadError && (
        <div className={styles.layout}>
          <div className={styles.side}>
            <Section title="Sprints" ariaLabel="Sprints del proyecto">
              {sprints.length === 0 ? (
                <EmptyState title="Aún no hay sprints" description="Planifica el primero con el formulario de abajo." />
              ) : (
                <ul className={styles.sprintList} aria-label="Lista de sprints">
                  {sprints.map((sprint) => (
                    <li key={sprint.id}>
                      <button
                        type="button"
                        className={sprint.id === selectedId ? styles.sprintSelected : styles.sprintItem}
                        aria-pressed={sprint.id === selectedId}
                        onClick={() => setSelectedId(sprint.id)}
                      >
                        <span className={styles.sprintName}>{sprint.name}</span>
                        <Lozenge tone={statusTones[sprint.status]}>{statusLabels[sprint.status]}</Lozenge>
                      </button>
                    </li>
                  ))}
                </ul>
              )}
            </Section>

            <Section title="Nuevo sprint">
              <Space orientation="vertical" className={styles.form}>
                <Input placeholder="Nombre" aria-label="Nombre del sprint" value={name} maxLength={80} onChange={(event) => setName(event.target.value)} />
                <Input placeholder="Objetivo" aria-label="Objetivo del sprint" value={goal} maxLength={500} onChange={(event) => setGoal(event.target.value)} />
                <Input type="date" aria-label="Fecha de inicio" value={startDate} onChange={(event) => setStartDate(event.target.value)} />
                <Input type="date" aria-label="Fecha de fin" value={endDate} onChange={(event) => setEndDate(event.target.value)} />
                <Button type="primary" onClick={create}>Planificar sprint</Button>
              </Space>
            </Section>
          </div>

          <div className={styles.main}>
            {!selected ? (
              <Section>
                <EmptyState title="Selecciona un sprint" description="Elige un sprint de la lista o planifica uno nuevo." />
              </Section>
            ) : (
              <>
                <Section
                  title={selected.name}
                  description={selected.goal || 'Sin objetivo definido.'}
                  actions={
                    <Space wrap>
                      {selected.status === 'planned' && (
                        <Button type="primary" onClick={() => run(() => sprintRepo.start(id, selected.id), selected.id)}>Iniciar sprint</Button>
                      )}
                      {selected.status === 'active' && (
                        <Space wrap>
                          <Radio.Group value={closeMode} onChange={(event) => setCloseMode(event.target.value)} aria-label="Destino de las incidencias no cerradas">
                            <Radio value="backlog">Al backlog</Radio>
                            <Radio value="sprint">A otro sprint</Radio>
                          </Radio.Group>
                          {closeMode === 'sprint' && (
                            <Select
                              aria-label="Sprint destino"
                              placeholder="Sprint planificado"
                              value={closeTarget}
                              onChange={setCloseTarget}
                              options={plannedTargets.map((sprint) => ({ value: sprint.id, label: sprint.name }))}
                              className={styles.targetSelect}
                            />
                          )}
                          <Button
                            danger
                            onClick={() => run(() => sprintRepo.close(id, selected.id, closeMode, closeMode === 'sprint' ? closeTarget : undefined))}
                            disabled={closeMode === 'sprint' && !closeTarget}
                          >
                            Cerrar sprint
                          </Button>
                        </Space>
                      )}
                    </Space>
                  }
                  footer={
                    <Lozenge tone={statusTones[selected.status]}>{statusLabels[selected.status]}</Lozenge>
                  }
                >
                  <div className={styles.stats}>
                    <StatTile label="Incidencias" value={issues.length} />
                    <StatTile label="Puntos" value={issues.reduce((total, issue) => total + (issue.storyPoints ?? 0), 0)} />
                    <StatTile label="Fechas" value={selected.startDate && selected.endDate ? `${selected.startDate} → ${selected.endDate}` : 'Sin fechas'} />
                  </div>
                </Section>

                <Section title="Incidencias del sprint">
                  {selected.status !== 'closed' && (
                    <Space wrap className={styles.addRow}>
                      <Select
                        aria-label="Incidencia a añadir"
                        placeholder="Añadir desde el backlog"
                        value={issueToAdd}
                        onChange={setIssueToAdd}
                        options={assignable.map((item) => ({ value: item.id, label: item.summary }))}
                        className={styles.targetSelect}
                      />
                      <Button disabled={!issueToAdd} onClick={() => {
                        const chosen = issueToAdd
                        if (!chosen) return
                        setIssueToAdd(undefined)
                        run(() => sprintRepo.assignIssue(id, selected.id, chosen), selected.id)
                      }}>Añadir</Button>
                    </Space>
                  )}
                  <DataTable<SprintIssue>
                    tableKey="sprint-issues"
                    columns={issueColumns}
                    data={issues}
                    rowKey="id"
                    emptyState={<EmptyState title="El sprint no tiene incidencias" description="Añade incidencias desde el backlog." />}
                    mobileRender={(issue) => (
                      <div className={styles.mobileIssue}>
                        <span className={styles.issueTitle}>{issue.title}</span>
                        <Lozenge tone="neutral">{issueStatusLabels[issue.status] ?? issue.status}</Lozenge>
                      </div>
                    )}
                  />
                </Section>

                <ChartCard
                  title="Burndown"
                  description="Trabajo restante frente a la línea ideal."
                  isEmpty={burndown.length === 0}
                  emptyText="El burndown necesita fechas de inicio y fin."
                  table={{
                    headers: ['Fecha', 'Ideal', 'Restante'],
                    rows: burndown.map((point) => [point.date, point.ideal, point.remaining]),
                  }}
                >
                  <BurndownChart points={burndown} />
                </ChartCard>
              </>
            )}

            <Section title="Velocidad (sprints cerrados)">
              {velocity.length === 0 ? (
                <EmptyState title="Aún no hay sprints cerrados" description="La velocidad aparecerá al cerrar el primer sprint." />
              ) : (
                <ul className={styles.velocityList}>
                  {velocity.map((point) => (
                    <li key={point.sprintId} className={styles.velocityItem}>
                      <span>{point.name}</span>
                      <strong>{point.points} puntos</strong>
                    </li>
                  ))}
                </ul>
              )}
            </Section>
          </div>
        </div>
      )}
    </PageContainer>
  )
}
