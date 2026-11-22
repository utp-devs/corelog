import {
  Alert,
  Button,
  Card,
  Empty,
  Skeleton,
  Space,
  Tag,
  Typography,
} from 'antd'
import { ArrowLeftOutlined } from '@ant-design/icons'
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
import { Link, useNavigate, useParams } from 'react-router-dom'

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

function DroppableColumn({ column, children }: { column: BoardColumn; children: ReactNode }) {
  const full = isColumnFull(column)
  const { setNodeRef, isOver } = useDroppable({ id: column.id, disabled: full })
  const classes = [styles.column, isOver && !full ? styles.columnOver : '', full ? styles.columnFull : ''].filter(Boolean).join(' ')
  return (
    <section ref={setNodeRef} className={classes} aria-labelledby={`column-${column.id}`}>
      <div className={styles.columnHeader}>
        <Typography.Title level={5} id={`column-${column.id}`} className={styles.columnTitle}>
          {column.name}
        </Typography.Title>
        <Tag color={full ? 'error' : undefined}>
          {column.wipLimit !== undefined ? `${column.count} / ${column.wipLimit}` : column.count}
        </Tag>
      </div>
      {children}
    </section>
  )
}

export function ProjectBoardPage() {
  const { id = '' } = useParams()
  const navigate = useNavigate()
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

  return (
    <div>
      <PageHeader
        title={currentResult?.project ? `${currentResult.project.key} · ${currentResult.project.name}` : 'Tablero del proyecto'}
        description="Arrastra una incidencia a otra columna para cambiar su estado."
        actions={<Button icon={<ArrowLeftOutlined />} onClick={() => navigate('/projects')}>Proyectos</Button>}
      />
      {moveError && (
        <Alert type="warning" showIcon closable message={moveError} onClose={() => setMoveError(undefined)} className={styles.alert} />
      )}
      {!currentResult ? <Skeleton active paragraph={{ rows: 5 }} /> : currentResult.error ? <Alert type="error" showIcon message={currentResult.error} /> : board?.columns.length ? (
        <DndContext sensors={sensors} onDragStart={handleDragStart} onDragEnd={handleDragEnd} onDragCancel={() => setActiveCardId(null)}>
          <div className={styles.board} aria-label={`Tablero ${board.name}`}>
            {board.columns.map((column) => (
              <DroppableColumn key={column.id} column={column}>
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
