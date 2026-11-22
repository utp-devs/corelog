import { Alert, Button, Card, Empty, Skeleton, Space, Tag, Typography } from 'antd'
import { ArrowLeftOutlined } from '@ant-design/icons'
import { useEffect, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'

import { BoardHttpRepository } from '@/adapters/out/http/board-http-repository'
import { ProjectHttpRepository } from '@/adapters/out/http/project-http-repository'
import type { Project } from '@/core/projects/domain/project'
import type { ProjectBoard } from '@/core/projects/domain/board'
import { PageHeader } from '@/shared/ui/PageHeader'
import { TicketBadge } from '@/adapters/in/ui/tickets/TicketBadge'
import type { TicketPriority } from '@/core/tickets/domain/ticket'
import styles from './ProjectBoardPage.module.css'

export function ProjectBoardPage() {
  const { id = '' } = useParams()
  const navigate = useNavigate()
  const [result, setResult] = useState<{
    projectId: string
    project?: Project
    board?: ProjectBoard
    error?: string
  } | null>(null)

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

  return (
    <div>
      <PageHeader
        title={currentResult?.project ? `${currentResult.project.key} · ${currentResult.project.name}` : 'Tablero del proyecto'}
        description="Consulta las incidencias agrupadas por estado."
        actions={<Button icon={<ArrowLeftOutlined />} onClick={() => navigate('/projects')}>Proyectos</Button>}
      />
      {!currentResult ? <Skeleton active paragraph={{ rows: 5 }} /> : currentResult.error ? <Alert type="error" showIcon message={currentResult.error} /> : currentResult.board?.columns.length ? (
        <div className={styles.board} aria-label={`Tablero ${currentResult.board.name}`}>
          {currentResult.board.columns.map((column) => (
            <section className={styles.column} key={column.id} aria-labelledby={`column-${column.id}`}>
              <div className={styles.columnHeader}>
                <Typography.Title level={5} id={`column-${column.id}`} className={styles.columnTitle}>
                  {column.name}
                </Typography.Title>
                <Tag>{column.count}</Tag>
              </div>
              <div className={styles.cards}>
                {column.cards.length === 0 ? <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="Sin incidencias" /> : column.cards.map((card) => (
                  <Card key={card.id} size="small" className={styles.issueCard}>
                    <Space orientation="vertical" size="small" className={styles.cardContent}>
                      <Typography.Text type="secondary">{card.id.slice(0, 8)} · {card.type}</Typography.Text>
                      <Link to={`/issues/${card.id}`} className={styles.summary}>{card.summary}</Link>
                      <Space wrap size={[6, 6]}>
                        <TicketBadge kind="priority" value={card.priority as TicketPriority} />
                        {card.storyPoints !== undefined && <Tag>{card.storyPoints} puntos</Tag>}
                      </Space>
                      <Typography.Text type="secondary">Responsable: {card.assignee || 'Sin asignar'}</Typography.Text>
                    </Space>
                  </Card>
                ))}
              </div>
            </section>
          ))}
        </div>
      ) : <Empty description="El proyecto todavía no tiene columnas de tablero." />}
    </div>
  )
}
