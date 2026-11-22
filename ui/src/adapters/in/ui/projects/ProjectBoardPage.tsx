import {
  Alert,
  Card,
  Empty,
  Input,
  InputNumber,
  Select,
  Skeleton,
  Space,
  Switch,
  Tag,
  Typography,
} from 'antd'
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
import type { TicketPriority } from '@/core/tickets/domain/ticket'
import { PageHeader } from '@/shared/ui/PageHeader'
import { PriorityTag } from '@/shared/ui/PriorityTag'
import styles from './ProjectBoardPage.module.css'

// Mensajes en español para los errores de mover una tarjeta (permiso, límite WIP y flujo).
function moveErrorMessage(error: unknown): string {
  const status = (error as { response?: { status?: number } }).response?.status
  if (status === 403) return 'No tienes permiso para mover incidencias en este proyecto.'
  if (status === 409) return 'La columna destino alcanzó su límite WIP o el flujo no permite ese cambio.'
  return 'No se pudo mover la incidencia. Intenta de nuevo.'
}

// Una columna está llena cuando tiene límite WIP y ya alcanza ese límite.
// Mensajes en español para guardar el límite WIP de una columna.
function wipErrorMessage(error: unknown): string {
  const status = (error as { response?: { status?: number } }).response?.status
  if (status === 403) return 'Solo el lead del proyecto o el administrador pueden cambiar el límite WIP.'
  return 'No se pudo guardar el límite WIP. Intenta de nuevo.'
}

function isColumnFull(column: BoardColumn): boolean {
  return column.wipLimit !== undefined && column.count >= column.wipLimit
}

function BoardCardView({ card, dragging = false }: { card: BoardCard; dragging?: boolean }) {
  return (
    <Card size="small" className={dragging ? styles.cardPreview : styles.issueCard}>
      <Space orientation="vertical" size="small" className={styles.cardContent}>
        <Typography.Text type="secondary">{card.id.slice(0, 8)} · {card.type}</Typography.Text>
        <Link to={`/issues/${card.id}`} className={styles.summary}>{card.summary}</Link>
        <Space wrap size={[6, 6]}>
          <PriorityTag priority={card.priority as TicketPriority} />
          {card.storyPoints !== undefined && <Tag>{card.storyPoints} puntos</Tag>}
        </Space>
        <Typography.Text type="secondary">Responsable: {card.assignee || 'Sin asignar'}</Typography.Text>
      </Space>
    </Card>
  )
}

function DraggableCard({ card }: { card: BoardCard }) {
  const { attributes, listeners, setNodeRef, isDragging } = useDraggable({ id: card.id })
  return (
    <div
      ref={setNodeRef}
      className={isDragging ? styles.cardDragging : undefined}
      {...attributes}
      {...listeners}
    >
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
      <div className={styles.columnHeader}>
        <Typography.Title level={5} id={`column-${column.id}`} className={styles.columnTitle}>
          {column.name}
        </Typography.Title>
        <Space size={[6, 6]} wrap>
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
          <Tag color={full ? 'error' : undefined}>
            {column.wipLimit !== undefined ? `${column.count} / ${column.wipLimit}` : column.count}
          </Tag>
        </Space>
      </div>
      {children}
    </section>
  )
}

export function ProjectBoardPage() {
  const { id = '' } = useParams()
  const [result, setResult] = useState<{
    projectId: string
    project?: Project
    board?: ProjectBoard
    error?: string
  } | null>(null)
  const [activeCardId, setActiveCardId] = useState<string | null>(null)
  const [moveError, setMoveError] = useState<string | undefined>(undefined)

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
      .catch(() => {
        if (active) setResult({
          projectId: id,
          error: 'No se pudo cargar el tablero. Verifica que tengas acceso al proyecto e intenta de nuevo.',
        })
      })
    return () => { active = false }
  }, [id])

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

  const handleDragStart = (event: DragStartEvent) => {
    setActiveCardId(String(event.active.id))
  }

  const handleDragEnd = (event: DragEndEvent) => {
    setActiveCardId(null)
    const issueId = String(event.active.id)
    const targetColumnId = event.over ? String(event.over.id) : null
    if (!targetColumnId || !board) return
    const source = board.columns.find((column) => column.cards.some((card) => card.id === issueId))
    if (!source || source.id === targetColumnId) return

    setMoveError(undefined)
    new BoardHttpRepository()
      .moveCard(id, issueId, targetColumnId)
      .then((updated) => setResult((previous) => (previous && previous.projectId === id ? { ...previous, board: updated } : previous)))
      .catch((error: unknown) => setMoveError(moveErrorMessage(error)))
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

  return (
    <div>
      <PageHeader title="Tablero" description="Arrastra una incidencia a otra columna para cambiar su estado." />
      {moveError && (
        <Alert type="warning" showIcon closable message={moveError} onClose={() => setMoveError(undefined)} className={styles.alert} />
      )}
      {!currentResult ? <Skeleton active paragraph={{ rows: 5 }} /> : currentResult.error ? <Alert type="error" showIcon message={currentResult.error} /> : board?.columns.length ? (
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
          <div className={styles.board} aria-label={`Tablero ${board.name}`}>
            {visibleColumns.map((column) => (
              <DroppableColumn key={column.id} column={column} canManage={board.canManageBoard} onSaveWip={saveWip}>
                <div className={styles.cards}>
                  {column.cards.length === 0 ? <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="Sin incidencias" /> : column.cards.map((card) => (
                    <DraggableCard key={card.id} card={card} />
                  ))}
                </div>
              </DroppableColumn>
            ))}
          </div>
          <DragOverlay>{activeCard ? <BoardCardView card={activeCard} dragging /> : null}</DragOverlay>
        </DndContext>
      ) : <Empty description="El proyecto todavía no tiene columnas de tablero." />}
    </div>
  )
}
