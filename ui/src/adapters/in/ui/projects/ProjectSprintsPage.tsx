import { Alert, Button, Empty, Input, Radio, Select, Skeleton, Space, Statistic, Tag, Typography } from 'antd'
import { ArrowLeftOutlined } from '@ant-design/icons'
import { useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'

import { BacklogHttpRepository } from '@/adapters/out/http/backlog-http-repository'
import { ProjectHttpRepository } from '@/adapters/out/http/project-http-repository'
import { SprintHttpRepository } from '@/adapters/out/http/sprint-http-repository'
import type { BacklogItem } from '@/core/projects/domain/backlog'
import type { BurndownPoint, Sprint, SprintIssue, SprintStatus, VelocityPoint } from '@/core/projects/domain/sprint'
import type { Project } from '@/core/projects/domain/project'
import { PageHeader } from '@/shared/ui/PageHeader'
import { BurndownChart } from './BurndownChart'
import styles from './ProjectSprintsPage.module.css'

const statusLabels: Record<SprintStatus, string> = { planned: 'Planificado', active: 'Activo', closed: 'Cerrado' }
const statusColors: Record<SprintStatus, string | undefined> = { planned: undefined, active: 'processing', closed: 'default' }

// Mensajes en español para las acciones de sprint, segun el codigo de la API.
function sprintErrorMessage(error: unknown): string {
  const status = (error as { response?: { status?: number } }).response?.status
  if (status === 403) return 'Solo el lead del proyecto o el administrador pueden gestionar sprints.'
  if (status === 409) return 'Esa acción no está permitida en el estado actual del sprint o ya hay otro sprint activo.'
  if (status === 422) return 'Revisa los datos del sprint: fechas en formato AAAA-MM-DD y fin posterior al inicio.'
  return 'No se pudo completar la acción. Intenta de nuevo.'
}

export function ProjectSprintsPage() {
  const { id = '' } = useParams()
  const navigate = useNavigate()
  const [project, setProject] = useState<Project | undefined>(undefined)
  const [sprints, setSprints] = useState<Sprint[] | undefined>(undefined)
  const [velocity, setVelocity] = useState<VelocityPoint[]>([])
  const [loadError, setLoadError] = useState<string | undefined>(undefined)
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
    void Promise.all([new ProjectHttpRepository().get(id), sprintRepo.list(id), sprintRepo.velocity(id)])
      .then(([loadedProject, loadedSprints, loadedVelocity]) => {
        if (!active) return
        setProject(loadedProject)
        setSprints(loadedSprints)
        setVelocity(loadedVelocity)
        const first = loadedSprints.find((sprint) => sprint.status === 'active') ?? loadedSprints.find((sprint) => sprint.status === 'planned') ?? loadedSprints[0]
        setSelectedId(first?.id)
      })
      .catch(() => {
        if (active) setLoadError('No se pudieron cargar los sprints. Verifica que tengas acceso al proyecto e intenta de nuevo.')
      })
    return () => { active = false }
    // eslint-disable-next-line react-hooks/exhaustive-deps -- la carga inicial depende solo del proyecto
  }, [id])

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

  if (loadError) {
    return (
      <div>
        <PageHeader title="Sprints" description="Planifica y sigue las iteraciones del proyecto." />
        <Alert type="error" showIcon message={loadError} />
      </div>
    )
  }

  return (
    <div>
      <PageHeader
        title={project ? `${project.key} · Sprints` : 'Sprints del proyecto'}
        description="Planifica el trabajo, activa un sprint y ciérralo al terminar."
        actions={<Button icon={<ArrowLeftOutlined />} onClick={() => navigate(`/projects/${id}/backlog`)}>Backlog</Button>}
      />
      {actionError && <Alert type="warning" showIcon closable message={actionError} onClose={() => setActionError(undefined)} className={styles.alert} />}

      {!sprints ? <Skeleton active paragraph={{ rows: 6 }} /> : (
        <div className={styles.layout}>
          <section className={styles.sidebar} aria-label="Sprints del proyecto">
            <Typography.Title level={5}>Sprints</Typography.Title>
            {sprints.length === 0 ? <Empty description="Aún no hay sprints." /> : (
              <ul className={styles.sprintList}>
                {sprints.map((sprint) => (
                  <li key={sprint.id}>
                    <button
                      type="button"
                      className={sprint.id === selectedId ? styles.sprintSelected : styles.sprintItem}
                      aria-pressed={sprint.id === selectedId}
                      onClick={() => setSelectedId(sprint.id)}
                    >
                      <span>{sprint.name}</span>
                      <Tag color={statusColors[sprint.status]}>{statusLabels[sprint.status]}</Tag>
                    </button>
                  </li>
                ))}
              </ul>
            )}

            <Typography.Title level={5}>Nuevo sprint</Typography.Title>
            <Space orientation="vertical" className={styles.form}>
              <Input placeholder="Nombre" aria-label="Nombre del sprint" value={name} maxLength={80} onChange={(event) => setName(event.target.value)} />
              <Input placeholder="Objetivo" aria-label="Objetivo del sprint" value={goal} maxLength={500} onChange={(event) => setGoal(event.target.value)} />
              <Input type="date" aria-label="Fecha de inicio" value={startDate} onChange={(event) => setStartDate(event.target.value)} />
              <Input type="date" aria-label="Fecha de fin" value={endDate} onChange={(event) => setEndDate(event.target.value)} />
              <Button type="primary" onClick={create}>Planificar sprint</Button>
            </Space>
          </section>

          <section className={styles.main} aria-label="Detalle del sprint">
            {!selected ? <Empty description="Selecciona o planifica un sprint." /> : (
              <>
                <div className={styles.header}>
                  <div>
                    <Typography.Title level={4} className={styles.title}>{selected.name}</Typography.Title>
                    <Typography.Text type="secondary">{selected.goal || 'Sin objetivo definido.'}</Typography.Text>
                  </div>
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
                </div>

                <Space wrap size="large" className={styles.stats}>
                  <Statistic title="Incidencias" value={issues.length} />
                  <Statistic title="Puntos" value={issues.reduce((total, issue) => total + (issue.storyPoints ?? 0), 0)} />
                  <Statistic title="Fechas" value={selected.startDate && selected.endDate ? `${selected.startDate} → ${selected.endDate}` : 'Sin fechas'} />
                </Space>

                <Typography.Title level={5}>Burndown</Typography.Title>
                {burndown.length === 0 ? (
                  <Empty description="El burndown necesita fechas de inicio y fin." />
                ) : (
                  <BurndownChart points={burndown} />
                )}

                <Typography.Title level={5}>Incidencias del sprint</Typography.Title>
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
                {issues.length === 0 ? <Empty description="El sprint no tiene incidencias." /> : (
                  <ul className={styles.issueList} aria-label="Incidencias del sprint">
                    {issues.map((issue) => (
                      <li key={issue.id} className={styles.issueRow}>
                        <span className={styles.issueTitle}>{issue.title}</span>
                        <Space size="small">
                          <Tag>{issue.status}</Tag>
                          {issue.storyPoints !== undefined && <Tag>{issue.storyPoints} puntos</Tag>}
                          {selected.status !== 'closed' && (
                            <Button size="small" aria-label={`Quitar ${issue.title}`} onClick={() => run(() => sprintRepo.removeIssue(id, selected.id, issue.id), selected.id)}>
                              Quitar
                            </Button>
                          )}
                        </Space>
                      </li>
                    ))}
                  </ul>
                )}
              </>
            )}

            <Typography.Title level={5}>Velocidad (sprints cerrados)</Typography.Title>
            {velocity.length === 0 ? <Empty description="Aún no hay sprints cerrados." /> : (
              <ul className={styles.velocityList}>
                {velocity.map((point) => (
                  <li key={point.sprintId}>{point.name}: <strong>{point.points}</strong> puntos</li>
                ))}
              </ul>
            )}
          </section>
        </div>
      )}
    </div>
  )
}
