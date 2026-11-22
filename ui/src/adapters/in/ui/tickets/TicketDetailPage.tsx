import { App, Button, Card, DatePicker, Descriptions, Input, InputNumber, List, Select, Skeleton, Space, Tabs, Typography } from 'antd'
import TextArea from 'antd/es/input/TextArea'
import dayjs from 'dayjs'
import { useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'

import { canTransitionTo, type IssueType, type TicketStatus } from '@/core/tickets/domain/ticket'
import { useTicketsStore } from '@/adapters/in/state/tickets.store'
import { useTeammateSearch } from './hooks/useTeammateSearch'
import { TicketBadge } from './TicketBadge'
import styles from './TicketDetailPage.module.css'
import { PageHeader } from '@/shared/ui/PageHeader'

const allStatuses: TicketStatus[] = ['open', 'in_progress', 'resolved', 'closed']

export function TicketDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const { message } = App.useApp()

  const selected = useTicketsStore((s) => s.selected)
  const status = useTicketsStore((s) => s.status)
  const fetchTicket = useTicketsStore((s) => s.fetchTicket)
  const updateStatus = useTicketsStore((s) => s.updateStatus)
  const assign = useTicketsStore((s) => s.assign)
  const updateMetadata = useTicketsStore((s) => s.updateMetadata)
  const comments = useTicketsStore((s) => s.comments)
  const activities = useTicketsStore((s) => s.activities)
  const fetchComments = useTicketsStore((s) => s.fetchComments)
  const createComment = useTicketsStore((s) => s.createComment)
  const deleteComment = useTicketsStore((s) => s.deleteComment)
  const fetchActivities = useTicketsStore((s) => s.fetchActivities)
  const links = useTicketsStore((s) => s.links)
  const fetchLinks = useTicketsStore((s) => s.fetchLinks)
  const createLink = useTicketsStore((s) => s.createLink)
  const deleteLink = useTicketsStore((s) => s.deleteLink)

  const [assigneeId, setAssigneeId] = useState('')
  const [issueType, setIssueType] = useState<IssueType>(selected?.issueType ?? 'task')
  const [dueDate, setDueDate] = useState(selected?.dueDate ? dayjs(selected.dueDate) : null)
  const [storyPoints, setStoryPoints] = useState<number | null>(selected?.storyPoints ?? null)
  const [commentBody, setCommentBody] = useState('')
  const [linkTargetId, setLinkTargetId] = useState('')
  const [linkKind, setLinkKind] = useState<'blocks' | 'relates' | 'duplicates'>('relates')
  const teammateSearch = useTeammateSearch()

  useEffect(() => {
    if (id) fetchTicket(id)
  }, [id, fetchTicket])

  useEffect(() => {
    if (!id) return
    void fetchComments(id)
    void fetchActivities(id)
    void fetchLinks(id)
  }, [id, fetchComments, fetchActivities, fetchLinks])

  if (status === 'loading' || !selected) {
    return <Skeleton active />
  }

  const nextStatuses = allStatuses.filter((s) => canTransitionTo(selected.status, s))

  const handleStatusChange = async (next: TicketStatus) => {
    try {
      await updateStatus(selected.id, next)
      message.success('Estado actualizado')
    } catch (error) {
      message.error(error instanceof Error ? error.message : 'No se pudo actualizar el estado')
    }
  }

  const handleAssign = async () => {
    if (!assigneeId) return
    try {
      await assign(selected.id, assigneeId)
      message.success('Incidencia asignada')
      setAssigneeId('')
    } catch (error) {
      message.error(error instanceof Error ? error.message : 'No se pudo asignar la incidencia')
    }
  }

  return (
    <div className={styles.wrapper}>
      <PageHeader
        title={selected.title}
        description="Detalle de la incidencia"
        actions={
          <Button onClick={() => navigate('/issues')}>
            Volver a incidencias
          </Button>
        }
      />

      <Card title="Vinculos">
        <Space wrap>
          <Input
            value={linkTargetId}
            onChange={(event) => setLinkTargetId(event.target.value)}
            placeholder="UUID de incidencia"
            style={{ width: 280 }}
          />
          <Select
            value={linkKind}
            onChange={setLinkKind}
            options={[
              { value: 'blocks', label: 'Bloquea' },
              { value: 'relates', label: 'Se relaciona con' },
              { value: 'duplicates', label: 'Duplica' },
            ]}
          />
          <Button
            type="primary"
            disabled={!linkTargetId}
            onClick={async () => {
              try {
                await createLink(selected.id, linkTargetId, linkKind)
                setLinkTargetId('')
                message.success('Vinculo creado')
              } catch (error) {
                message.error(error instanceof Error ? error.message : 'No se pudo crear el vinculo')
              }
            }}
          >
            Vincular
          </Button>
        </Space>
        <List
          dataSource={links}
          locale={{ emptyText: 'Aun no hay vinculos' }}
          renderItem={(link) => (
            <List.Item
              actions={[
                <Button
                  key="delete"
                  danger
                  size="small"
                  onClick={() => void deleteLink(selected.id, link.id)}
                >
                  Eliminar
                </Button>,
              ]}
            >
              {link.kind} - {link.sourceIssueId === selected.id ? link.targetIssueId : link.sourceIssueId}
            </List.Item>
          )}
        />
      </Card>

      <Card>
        <Typography.Paragraph>{selected.description}</Typography.Paragraph>

        <Descriptions column={1} bordered size="small">
          <Descriptions.Item label="Estado">
            <TicketBadge kind="status" value={selected.status} />
          </Descriptions.Item>
          <Descriptions.Item label="Prioridad">
            <TicketBadge kind="priority" value={selected.priority} />
          </Descriptions.Item>
          <Descriptions.Item label="Solicitante">{selected.requesterId}</Descriptions.Item>
          <Descriptions.Item label="Responsable">{selected.assigneeId ?? 'Sin asignar'}</Descriptions.Item>
          <Descriptions.Item label="Creado">{new Date(selected.createdAt).toLocaleString('es-ES')}</Descriptions.Item>
          <Descriptions.Item label="Actualizado">{new Date(selected.updatedAt).toLocaleString('es-ES')}</Descriptions.Item>
        </Descriptions>
      </Card>

      <Card title="Cambiar estado">
        <Space>
          <Select
            className={styles.statusSelect}
            placeholder="Seleccionar siguiente estado"
            disabled={nextStatuses.length === 0}
            options={nextStatuses.map((s) => ({
              value: s,
              label: s === 'open' ? 'Abierto' : s === 'in_progress' ? 'En progreso' : s === 'resolved' ? 'Resuelto' : 'Cerrado',
            }))}
            onChange={handleStatusChange}
          />
          {nextStatuses.length === 0 && <Typography.Text type="secondary">No hay transiciones disponibles</Typography.Text>}
        </Space>
      </Card>

      <Card title="Metadatos">
        <Space wrap>
          <Select<IssueType>
            value={issueType}
            onChange={setIssueType}
            options={[
              { value: 'task', label: 'Tarea' },
              { value: 'bug', label: 'Error' },
              { value: 'story', label: 'Historia' },
              { value: 'epic', label: 'Epic' },
              { value: 'subtask', label: 'Subtarea' },
            ]}
          />
          <DatePicker
            value={dueDate}
            onChange={(value) => setDueDate(value)}
            placeholder="Fecha limite"
          />
          <InputNumber
            value={storyPoints}
            onChange={setStoryPoints}
            min={0}
            max={999}
            placeholder="Puntos"
          />
          <Button
            type="primary"
            onClick={async () => {
              try {
                await updateMetadata(selected.id, {
                  issueType,
                  dueDate: dueDate?.toISOString(),
                  storyPoints: storyPoints ?? undefined,
                })
                message.success('Metadatos actualizados')
              } catch (error) {
                message.error(error instanceof Error ? error.message : 'No se pudieron actualizar los metadatos')
              }
            }}
          >
            Guardar
          </Button>
        </Space>
      </Card>

      <Card title="Asignar">
        <Space>
          <Select
            showSearch
            filterOption={false}
            placeholder="Buscar un compañero por nombre o correo"
            className={styles.assigneeInput}
            loading={teammateSearch.isLoading}
            notFoundContent={teammateSearch.isLoading ? 'Buscando...' : 'No se encontraron compañeros'}
            onSearch={teammateSearch.search}
            onChange={(value) => setAssigneeId(value)}
            value={assigneeId || undefined}
            options={teammateSearch.results.map((u) => ({ value: u.id, label: `${u.name} (${u.email})` }))}
          />
          <Button type="primary" onClick={handleAssign} disabled={!assigneeId}>
            Asignar
          </Button>
        </Space>
      </Card>

      <Card>
        <Tabs
          items={[
            {
              key: 'comments',
              label: 'Comentarios',
              children: (
                <Space direction="vertical" className={styles.collaboration}>
                  <TextArea
                    value={commentBody}
                    onChange={(event) => setCommentBody(event.target.value)}
                    placeholder="Escribe un comentario. Usa @usuario para mencionar."
                    maxLength={5000}
                    rows={3}
                  />
                  <Button
                    type="primary"
                    disabled={!commentBody.trim()}
                    onClick={async () => {
                      try {
                        await createComment(selected.id, commentBody)
                        setCommentBody('')
                        message.success('Comentario agregado')
                      } catch (error) {
                        message.error(error instanceof Error ? error.message : 'No se pudo agregar el comentario')
                      }
                    }}
                  >
                    Comentar
                  </Button>
                  <List
                    dataSource={comments}
                    locale={{ emptyText: 'Aun no hay comentarios' }}
                    renderItem={(comment) => (
                      <List.Item
                        actions={[
                          <Button
                            key="delete"
                            danger
                            size="small"
                            onClick={async () => {
                              try {
                                await deleteComment(selected.id, comment.id)
                                message.success('Comentario eliminado')
                              } catch (error) {
                                message.error(error instanceof Error ? error.message : 'No se pudo eliminar')
                              }
                            }}
                          >
                            Eliminar
                          </Button>,
                        ]}
                      >
                        <List.Item.Meta
                          title={comment.authorId}
                          description={comment.body}
                        />
                      </List.Item>
                    )}
                  />
                </Space>
              ),
            },
            {
              key: 'activity',
              label: 'Actividad',
              children: (
                <List
                  dataSource={activities}
                  locale={{ emptyText: 'Aun no hay actividad' }}
                  renderItem={(activity) => (
                    <List.Item>
                      <Typography.Text>
                        {activity.action}
                        {activity.fieldName ? ` - ${activity.fieldName}` : ''}
                      </Typography.Text>
                    </List.Item>
                  )}
                />
              ),
            },
          ]}
        />
      </Card>
    </div>
  )
}
