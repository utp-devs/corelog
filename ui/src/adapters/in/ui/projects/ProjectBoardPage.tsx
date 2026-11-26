import { HolderOutlined } from '@ant-design/icons'
import { Alert, Input, InputNumber, Select, Skeleton, Switch, Typography } from 'antd'
import {
  DndContext,
  DragOverlay,
  KeyboardSensor,
  PointerSensor,
  useDraggable,
  useDroppable,
  useSensor,
  useSensors,
  type DragEndEvent,
  type DragStartEvent,
} from '@dnd-kit/core'
import { useEffect, useState, type ReactNode } from 'react'
import { Link, useParams } from 'react-router-dom'

import { useAuthStore } from '@/adapters/in/state/auth.store'
import { BoardHttpRepository } from '@/adapters/out/http/board-http-repository'
import { ProjectHttpRepository } from '@/adapters/out/http/project-http-repository'
import type { BoardCard, BoardColumn, ProjectBoard } from '@/core/projects/domain/board'
import type { Project } from '@/core/projects/domain/project'
import { httpStatusOf } from '@/core/shared/app-error'
import type { IssueType, TicketPriority } from '@/core/tickets/domain/ticket'
import { Avatar } from '@/shared/ui/Avatar'
import { EmptyState } from '@/shared/ui/EmptyState'
import { ErrorPage } from '@/shared/ui/ErrorPage'
import { ErrorState } from '@/shared/ui/ErrorState'
import { IssueKey } from '@/shared/ui/IssueKey'
import { Lozenge } from '@/shared/ui/Lozenge'
import { PageContainer } from '@/shared/ui/PageContainer'
import { PageHeader } from '@/shared/ui/PageHeader'
import { PriorityTag } from '@/shared/ui/PriorityTag'
import { accessVariantOf, type AccessVariant } from './projectLoadError'
import styles from './ProjectBoardPage.module.css'

const ISSUE_TYPES: readonly IssueType[] = ['task', 'bug', 'story', 'epic', 'subtask']

// Mensajes en español para los errores de mover una tarjeta (permiso, límite WIP y flujo).
function moveErrorMessage(error: unknown): string {
  const status = httpStatusOf(error)
  if (status === 403) return 'No tienes permiso para mover incidencias en este proyecto.'
  if (status === 409) return 'La columna destino alcanzó su límite WIP o el flujo no permite ese cambio.'
  return 'No se pudo mover la incidencia. Intenta de nuevo.'
}

// Mensajes en español para guardar el límite WIP de una columna.
function wipErrorMessage(error: unknown): string {
  const status = httpStatusOf(error)
  if (status === 403) return 'Solo el lead del proyecto o el administrador pueden cambiar el límite WIP.'
  return 'No se pudo guardar el límite WIP. Intenta de nuevo.'
}

// Una columna está llena cuando tiene límite WIP y ya alcanza ese límite.
function isColumnFull(column: BoardColumn): boolean {
  return column.wipLimit !== undefined && column.count >= column.wipLimit
}

function asIssueType(value: string): IssueType | undefined {
  return ISSUE_TYPES.find((type) => type === value)
}

function shortId(id: string): string {
  return id.slice(0, 8)
}

function BoardCardView({ card, dragging = false }: { card: BoardCard; dragging?: boolean }) {
  return (
    <article className={dragging ? styles.cardPreview : styles.issueCard} data-testid={`board-card-${card.id}`}>
      <div className={styles.cardTop}>
        <IssueKey issueKey={shortId(card.id)} href={`/issues/${card.id}`} summary={card.summary} type={asIssueType(card.type)} />
      </div>
      <Link to={`/issues/${card.id}`} className={styles.summary} title={card.summary}>
        {card.summary}
      </Link>
      <div className={styles.cardMeta}>
        <PriorityTag priority={card.priority as TicketPriority} />
        {card.storyPoints !== undefined && <Lozenge>{card.storyPoints} puntos</Lozenge>}
      </div>
      <div className={styles.assignee}>
        {card.assignee ? (
          <>
            <Avatar id={card.assigneeId ?? card.id} name={card.assignee} size={20} />
            <span className={styles.assigneeName} title={card.assignee}>Responsable: {card.assignee}</span>
          </>
        ) : (
          <Typography.Text type="secondary">Sin asignar</Typography.Text>
        )}
      </div>
    </article>
  )
}

// Solo el asa de arrastre es draggable: el resto de la tarjeta (enlaces, copia, selector) no compite con el teclado.
function DraggableCard({ card }: { card: BoardCard }) {
  const { attributes, listeners, setNodeRef, isDragging } = useDraggable({ id: card.id })
  return (
    <div className={isDragging ? styles.cardDragging : styles.cardRow}>
      <button
        type="button"
        ref={setNodeRef}
        className={styles.handle}
        aria-label={`Arrastrar ${card.summary}`}
        {...attributes}
        {...listeners}
      >
        <HolderOutlined aria-hidden="true" />
      </button>
      <BoardCardView card={card} />
    </div>
  )
}

function DroppableColumn({ column, children, canManage, onSaveWip }: {
  column: BoardColumn
  children: ReactNode
  canManage: boolean
  onSaveWip: (columnId: string, raw: string) => void
}) {
  const full = isColumnFull(column)
  const { setNodeRef, isOver } = useDroppable({ id: column.id, disabled: full })
  const classes = [styles.column, isOver && !full ? styles.columnOver : '', full ? styles.columnFull : ''].filter(Boolean).join(' ')
  return (
    <section ref={setNodeRef} className={classes} aria-labelledby={`column-${column.id}`}>
      <header className={styles.columnHeader}>
        <h3 id={`column-${column.id}`} className={styles.columnTitle} title={column.name}>
          {column.name}
        </h3>
        <div className={styles.columnMeta}>
          {canManage && (
            <InputNumber
              key={`${column.id}-${column.wipLimit ?? 'none'}`}
              size="small"
              min={1}
              defaultValue={column.wipLimit}
              placeholder="Sin límite"
              aria-label={`Límite WIP de ${column.name}`}
              className={styles.wipInput}
              onBlur={(event) => onSaveWip(column.id, event.target.value)}
              onPressEnter={(event) => onSaveWip(column.id, event.currentTarget.value)}
            />
          )}
          <Lozenge tone={full ? 'danger' : 'neutral'}>
            {column.wipLimit !== undefined ? `${column.count} / ${column.wipLimit}` : String(column.count)}
          </Lozenge>
        </div>
      </header>
      <div className={styles.columnBody}>{children}</div>
    </section>
  )
}

export function ProjectBoardPage() {
  const { id = '' } = useParams()
  const [result, setResult] = useState<{
    projectId: string
    project?: Project
    board?: ProjectBoard
    error?: { variant: AccessVariant | null; message: string }
  } | null>(null)
  const [attempt, setAttempt] = useState(0)
  const [activeCardId, setActiveCardId] = useState<string | null>(null)
  const [moveError, setMoveError] = useState<string | undefined>(undefined)
  const [announcement, setAnnouncement] = useState('')

  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 5 } }),
    useSensor(KeyboardSensor),
  )

  useEffect(() => {
    let active = true
    const projects = new ProjectHttpRepository()
    const boards = new BoardHttpRepository()
    void Promise.all([projects.get(id), boards.getProjectBoard(id)])
      .then(([loadedProject, loadedBoard]) => {
        if (!active) return
        setResult({ projectId: id, project: loadedProject, board: loadedBoard })
      })
      .catch((failure: unknown) => {
        if (active) setResult({
          projectId: id,
          error: {
            variant: accessVariantOf(failure),
            message: 'No se pudo cargar el tablero. Verifica que tengas acceso al proyecto e intenta de nuevo.',
          },
        })
      })
    return () => { active = false }
  }, [id, attempt])

  const currentResult = result?.projectId === id ? result : null
  const board = currentResult?.board
  const currentUserId = useAuthStore((s) => s.user?.id)
  const [search, setSearch] = useState('')
  const [onlyMine, setOnlyMine] = useState(false)
  const [priorityFilter, setPriorityFilter] = useState<string | undefined>(undefined)

  // Filtros rapidos sobre las tarjetas ya cargadas: no generan peticiones.
  const matchesFilters = (card: BoardCard) => {
    if (onlyMine && card.assigneeId !== currentUserId) return false
    if (priorityFilter && card.priority !== priorityFilter) return false
    const term = search.trim().toLowerCase()
    if (term && !card.summary.toLowerCase().includes(term)) return false
    return true
  }
  const visibleColumns = (board?.columns ?? []).map((column) => ({ ...column, cards: column.cards.filter(matchesFilters) }))

  // Mueve una incidencia a otra columna (arrastre o selector de teclado) y reemplaza el tablero con la respuesta.
  const moveIssue = (issueId: string, targetColumnId: string) => {
    if (!board) return
    const source = board.columns.find((column) => column.cards.some((card) => card.id === issueId))
    if (!source || source.id === targetColumnId) return
    const target = board.columns.find((column) => column.id === targetColumnId)

    setMoveError(undefined)
    new BoardHttpRepository()
      .moveCard(id, issueId, targetColumnId)
      .then((updated) => {
        setResult((previous) => (previous && previous.projectId === id ? { ...previous, board: updated } : previous))
        setAnnouncement(`Incidencia movida a ${target?.name ?? 'otra columna'}`)
      })
      .catch((error: unknown) => setMoveError(moveErrorMessage(error)))
  }

  const handleDragStart = (event: DragStartEvent) => {
    setActiveCardId(String(event.active.id))
  }

  const handleDragEnd = (event: DragEndEvent) => {
    setActiveCardId(null)
    if (!event.over) return
    moveIssue(String(event.active.id), String(event.over.id))
  }

  const activeCard = board?.columns.flatMap((column) => column.cards).find((card) => card.id === activeCardId)

  // Fija el limite WIP de una columna. Cadena vacia quita el limite; solo se llama a la API si cambia.
  const saveWip = (columnId: string, raw: string) => {
    if (!board) return
    const column = board.columns.find((candidate) => candidate.id === columnId)
    const value = raw.trim() === '' ? null : Number(raw)
    if (value !== null && (!Number.isInteger(value) || value < 1)) {
      setMoveError('El límite WIP debe ser un número entero mayor que 0.')
      return
    }
    if (!column || (value ?? undefined) === column.wipLimit) return
    setMoveError(undefined)
    new BoardHttpRepository()
      .updateColumnWip(id, columnId, value)
      .then((updated) => setResult((previous) => (previous && previous.projectId === id ? { ...previous, board: updated } : previous)))
      .catch((error: unknown) => setMoveError(wipErrorMessage(error)))
  }

  if (currentResult?.error?.variant) {
    return (
      <PageContainer variant="fluid" pageName="board">
        <div className={styles.frame}>
          <ErrorPage variant={currentResult.error.variant} />
        </div>
      </PageContainer>
    )
  }

  const loading = !currentResult
  const ready = !loading && !currentResult?.error

  return (
    <PageContainer variant="fluid" pageName="board" ready={ready}>
      <div className={styles.frame}>
        <PageHeader title="Tablero" description="Arrastra una incidencia a otra columna, o usa su selector para cambiarla de columna." />
        <p className={styles.srOnly} role="status" aria-live="polite">{announcement}</p>
        {moveError && (
          <Alert type="warning" showIcon closable message={moveError} onClose={() => setMoveError(undefined)} className={styles.alert} />
        )}

        {loading && (
          <div className={styles.board} aria-busy="true" aria-label="Cargando tablero">
            {[0, 1, 2, 3].map((index) => (
              <div key={index} className={styles.column}>
                <Skeleton active paragraph={{ rows: 4 }} />
              </div>
            ))}
          </div>
        )}

        {currentResult?.error && (
          <ErrorState message={currentResult.error.message} onRetry={() => setAttempt((value) => value + 1)} />
        )}

        {!loading && !currentResult?.error && (board?.columns.length ? (
          <DndContext sensors={sensors} onDragStart={handleDragStart} onDragEnd={handleDragEnd} onDragCancel={() => setActiveCardId(null)}>
            <div className={styles.filters} role="search" aria-label="Filtrar tarjetas del tablero">
              <Input.Search
                allowClear
                placeholder="Buscar en el tablero"
                aria-label="Buscar en el tablero"
                value={search}
                onChange={(event) => setSearch(event.target.value)}
                className={styles.searchField}
              />
              <Select
                allowClear
                placeholder="Prioridad"
                aria-label="Filtrar por prioridad"
                value={priorityFilter}
                onChange={(value) => setPriorityFilter(value)}
                options={[
                  { value: 'critical', label: 'Crítica' },
                  { value: 'high', label: 'Alta' },
                  { value: 'medium', label: 'Media' },
                  { value: 'low', label: 'Baja' },
                ]}
                className={styles.priorityField}
              />
              <label className={styles.mineToggle}>
                <Switch size="small" checked={onlyMine} onChange={setOnlyMine} aria-label="Solo mis incidencias" />
                <Typography.Text>Solo mis incidencias</Typography.Text>
              </label>
            </div>
            <div className={styles.board} role="region" aria-label={`Tablero ${board.name}`}>
              {visibleColumns.map((column) => (
                <DroppableColumn key={column.id} column={column} canManage={board.canManageBoard} onSaveWip={saveWip}>
                  {column.cards.length === 0 ? (
                    <EmptyState title="Sin incidencias" description="Arrastra incidencias aquí." />
                  ) : (
                    <ul className={styles.cards} aria-label={`Incidencias en ${column.name}`}>
                      {column.cards.map((card) => (
                        <li key={card.id} className={styles.cardItem}>
                          <DraggableCard card={card} />
                          <Select
                            size="small"
                            aria-label={`Mover ${card.summary} a columna`}
                            placeholder="Mover a..."
                            value={null}
                            className={styles.moveSelect}
                            onChange={(targetColumnId: string) => moveIssue(card.id, targetColumnId)}
                            options={board.columns
                              .filter((candidate) => candidate.id !== column.id)
                              .map((candidate) => ({ value: candidate.id, label: candidate.name, disabled: isColumnFull(candidate) }))}
                          />
                        </li>
                      ))}
                    </ul>
                  )}
                </DroppableColumn>
              ))}
            </div>
            <DragOverlay>{activeCard ? <BoardCardView card={activeCard} dragging /> : null}</DragOverlay>
          </DndContext>
        ) : (
          <EmptyState title="Sin columnas" description="El proyecto todavía no tiene columnas de tablero." />
        ))}
      </div>
    </PageContainer>
  )
}
