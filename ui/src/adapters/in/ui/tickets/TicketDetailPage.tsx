import { Alert, App, Button, Card, DatePicker, Descriptions, Input, InputNumber, List, Select, Skeleton, Space, Tabs, Typography, Upload } from 'antd'
import TextArea from 'antd/es/input/TextArea'
import dayjs from 'dayjs'
import { useEffect, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'

import { canTransitionTo, type IssueType, type Ticket, type TicketPriority, type TicketStatus } from '@/core/tickets/domain/ticket'
import { useTicketsStore } from '@/adapters/in/state/tickets.store'
import { useTeammateSearch } from './hooks/useTeammateSearch'
import { PriorityTag } from '@/shared/ui/PriorityTag'
import { StatusTag } from '@/shared/ui/StatusTag'
import { IssuePicker } from './IssuePicker'
import styles from './TicketDetailPage.module.css'
import { PageHeader } from '@/shared/ui/PageHeader'
import type { IssueAttachment } from '@/core/tickets/domain/attachment'
import { AttachmentHttpRepository } from '@/adapters/out/http/attachment-http-repository'
import { extractErrorMessage } from '@/adapters/out/http/api-client'
import type { IssueWatcherState } from '@/core/tickets/domain/issue-watcher'
import { IssueWatcherHttpRepository } from '@/adapters/out/http/issue-watcher-http-repository'

const attachmentRepository = new AttachmentHttpRepository()
const watcherRepository = new IssueWatcherHttpRepository()

const allStatuses: TicketStatus[] = ['open', 'in_progress', 'resolved', 'closed']

type IssueEditorProps = {
  ticket: Ticket
  onSave: (details: {
    title: string
    description: string
    priority: TicketPriority
    issueType: IssueType
    parentId?: string
    dueDate?: string
    storyPoints?: number
  }) => Promise<void>
}

function IssueEditor({ ticket, onSave }: IssueEditorProps) {
  const [title, setTitle] = useState(ticket.title)
  const [description, setDescription] = useState(ticket.description)
  const [priority, setPriority] = useState<TicketPriority>(ticket.priority)
  const [issueType, setIssueType] = useState<IssueType>(ticket.issueType)
  const [parentId, setParentId] = useState(ticket.parentId ?? '')
  const [dueDate, setDueDate] = useState(ticket.dueDate ? dayjs(ticket.dueDate) : null)
  const [storyPoints, setStoryPoints] = useState<number | null>(ticket.storyPoints)

  return (
    <Card title="Metadatos">
      <Space direction="vertical" className={styles.editor}>
        <Input value={title} onChange={(event) => setTitle(event.target.value)} placeholder="Titulo" />
        <TextArea value={description} onChange={(event) => setDescription(event.target.value)} rows={4} placeholder="Descripcion" />
        <Space wrap>
          <Select<TicketPriority>
            value={priority}
            onChange={setPriority}
            options={[
              { value: 'low', label: 'Baja' },
              { value: 'medium', label: 'Media' },
              { value: 'high', label: 'Alta' },
              { value: 'critical', label: 'Crítica' },
            ]}
          />
          <Select<IssueType>
            value={issueType}
            onChange={setIssueType}
            options={[
              { value: 'task', label: 'Tarea' },
              { value: 'bug', label: 'Error' },
              { value: 'story', label: 'Historia' },
              { value: 'epic', label: 'Épica' },
              { value: 'subtask', label: 'Subtarea' },
            ]}
          />
          <Input value={parentId} onChange={(event) => setParentId(event.target.value)} placeholder="ID incidencia padre" className={styles.parentInput} />
          <DatePicker value={dueDate} onChange={(value) => setDueDate(value)} placeholder="Fecha limite" />
          <InputNumber value={storyPoints} onChange={setStoryPoints} min={0} max={999} placeholder="Puntos" />
          <Button
            type="primary"
            onClick={() => void onSave({
              title,
              description,
              priority,
              issueType,
              parentId: parentId || undefined,
              dueDate: dueDate?.toISOString(),
              storyPoints: storyPoints ?? undefined,
            })}
          >
            Guardar
          </Button>
        </Space>
      </Space>
    </Card>
  )
}

// Etiquetas en español para el tipo de vinculo entre incidencias.
const linkKindLabels: Record<string, string> = {
  blocks: 'Bloquea a',
  relates: 'Se relaciona con',
  duplicates: 'Duplica a',
}

export function TicketDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const { message } = App.useApp()

  const selected = useTicketsStore((s) => s.selected)
  const status = useTicketsStore((s) => s.status)
  const ticketError = useTicketsStore((s) => s.error)
  const fetchTicket = useTicketsStore((s) => s.fetchTicket)
  const updateStatus = useTicketsStore((s) => s.updateStatus)
  const assign = useTicketsStore((s) => s.assign)
  const updateDetails = useTicketsStore((s) => s.updateDetails)
  const comments = useTicketsStore((s) => s.comments)
  const commentsIssueId = useTicketsStore((s) => s.commentsIssueId)
  const activities = useTicketsStore((s) => s.activities)
  const activitiesIssueId = useTicketsStore((s) => s.activitiesIssueId)
  const fetchComments = useTicketsStore((s) => s.fetchComments)
  const createComment = useTicketsStore((s) => s.createComment)
  const deleteComment = useTicketsStore((s) => s.deleteComment)
  const fetchActivities = useTicketsStore((s) => s.fetchActivities)
  const links = useTicketsStore((s) => s.links)
  const linksIssueId = useTicketsStore((s) => s.linksIssueId)
  const fetchLinks = useTicketsStore((s) => s.fetchLinks)
  const createLink = useTicketsStore((s) => s.createLink)
  const deleteLink = useTicketsStore((s) => s.deleteLink)

  const [assigneeId, setAssigneeId] = useState('')
  const [commentBody, setCommentBody] = useState('')
  const [linkTargetId, setLinkTargetId] = useState('')
  const [linkKind, setLinkKind] = useState<'blocks' | 'relates' | 'duplicates'>('relates')
  const [attachments, setAttachments] = useState<IssueAttachment[]>([])
  const [attachmentsIssueId, setAttachmentsIssueId] = useState<string | null>(null)
  const [attachmentsError, setAttachmentsError] = useState<{ issueId: string; message: string } | null>(null)
  const attachmentsLoading = attachmentsIssueId !== selected?.id
  const attachmentLoadError = attachmentsError && attachmentsError.issueId === selected?.id ? attachmentsError.message : undefined
  const [collaborationLoad, setCollaborationLoad] = useState<{ issueId: string; error?: string } | null>(null)
  const collaborationLoading = !id || collaborationLoad?.issueId !== id
  const collaborationError = collaborationLoad && collaborationLoad.issueId === id ? collaborationLoad.error : undefined
  const [watcherResult, setWatcherResult] = useState<{ issueId: string; state?: IssueWatcherState; error?: string } | null>(null)
  const watcherLoading = watcherResult?.issueId !== id
  const watcherState = watcherResult?.issueId === selected?.id ? watcherResult?.state : undefined
  const watcherError = watcherResult?.issueId === selected?.id ? watcherResult?.error : undefined
  const [watcherActionLoading, setWatcherActionLoading] = useState(false)
  const teammateSearch = useTeammateSearch()

  useEffect(() => {
    if (id) fetchTicket(id)
  }, [id, fetchTicket])

  useEffect(() => {
    if (!id) return
    let active = true
    void Promise.allSettled([fetchComments(id), fetchActivities(id), fetchLinks(id)]).then((results) => {
      if (active) {
        setCollaborationLoad({
          issueId: id,
          error: results.some((result) => result.status === 'rejected')
            ? 'No se pudo cargar toda la información de colaboración.'
            : undefined,
        })
      }
    })
    void attachmentRepository.list(id).then((items) => {
      if (active) {
        setAttachments(items)
        setAttachmentsIssueId(id)
        setAttachmentsError(null)
      }
    }).catch((error: unknown) => {
      if (active) {
        setAttachments([])
        setAttachmentsIssueId(id)
        setAttachmentsError({
          issueId: id,
          message: extractErrorMessage(error, 'No se pudieron cargar los archivos'),
        })
      }
    })
    void watcherRepository.getState(id).then((state) => {
      if (active) setWatcherResult({ issueId: id, state })
    }).catch((error: unknown) => {
      if (active) setWatcherResult({ issueId: id, error: extractErrorMessage(error, 'No se pudo cargar el seguimiento') })
    })
    return () => { active = false }
  }, [id, fetchComments, fetchActivities, fetchLinks, message])

  if (status === 'loading' || (!selected && status === 'idle')) {
    return <Skeleton active />
  }

  if (status === 'error' || !selected || selected.id !== id) {
    return (
      <Alert
        type="error"
        showIcon
        message={ticketError ?? 'No se pudo encontrar la incidencia solicitada.'}
        action={<Button onClick={() => navigate('/issues')}>Volver a incidencias</Button>}
      />
    )
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
      await refreshWatcherState(selected.id)
      message.success('Incidencia asignada')
      setAssigneeId('')
    } catch (error) {
      message.error(error instanceof Error ? error.message : 'No se pudo asignar la incidencia')
    }
  }

  const refreshWatcherState = async (issueId: string) => {
    try {
      const state = await watcherRepository.getState(issueId)
      setWatcherResult({ issueId, state })
    } catch (error) {
      setWatcherResult({ issueId, error: extractErrorMessage(error, 'No se pudo actualizar el seguimiento') })
    }
  }

  const toggleWatcher = async () => {
    if (!watcherState) return
    setWatcherActionLoading(true)
    try {
      const state = watcherState.isWatching
        ? await watcherRepository.unwatch(selected.id)
        : await watcherRepository.watch(selected.id)
      setWatcherResult({ issueId: selected.id, state })
      message.success(state.isWatching ? 'Seguirás esta incidencia' : 'Dejaste de seguir esta incidencia')
    } catch (error) {
      message.error(extractErrorMessage(error, 'No se pudo actualizar el seguimiento'))
    } finally {
      setWatcherActionLoading(false)
    }
  }

  return (
    <div className={styles.wrapper}>
      <PageHeader
        title={selected.title}
        description="Detalle de la incidencia"
        actions={
          <Space wrap>
            <StatusTag status={selected.status} />
            <PriorityTag priority={selected.priority} />
            <Button onClick={() => navigate('/issues')}>
              Volver a incidencias
            </Button>
          </Space>
        }
      />

      <Tabs
        className={styles.detailTabs}
        key={selected.id}
        defaultActiveKey="detail"
        items={[
          {
            key: 'detail',
            label: 'Detalle',
            children: (
              <div className={styles.columns}>
                <div className={styles.mainColumn}>
      <Card title="Vinculos">
        {collaborationError && <Alert type="error" showIcon message={collaborationError} />}
        <Space wrap>
          <IssuePicker value={linkTargetId} onChange={setLinkTargetId} excludeId={selected.id} />
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
            disabled={collaborationLoading || !linkTargetId}
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
          loading={collaborationLoading}
          dataSource={collaborationLoading || linksIssueId !== selected.id || collaborationError ? [] : links}
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
              <Link to={`/issues/${(link.sourceIssueId === selected.id ? link.targetIssueId : link.sourceIssueId)}`}>
                {linkKindLabels[link.kind] ?? link.kind}: incidencia {(link.sourceIssueId === selected.id ? link.targetIssueId : link.sourceIssueId).slice(0, 8)}
              </Link>
            </List.Item>
          )}
        />
      </Card>

      <Card>
        <Typography.Paragraph>{selected.description}</Typography.Paragraph>

        <Descriptions column={1} bordered size="small">
          <Descriptions.Item label="Tipo">{issueTypeLabel(selected.issueType)}</Descriptions.Item>
          <Descriptions.Item label="Fecha limite">
            {selected.dueDate ? dayjs(selected.dueDate).format('DD/MM/YYYY') : 'Sin fecha limite'}
          </Descriptions.Item>
          <Descriptions.Item label="Puntos de historia">{selected.storyPoints ?? 'Sin estimar'}</Descriptions.Item>
          <Descriptions.Item label="Incidencia padre">{selected.parentId ?? 'Sin incidencia padre'}</Descriptions.Item>
          <Descriptions.Item label="Solicitante">{selected.requesterName}</Descriptions.Item>
          <Descriptions.Item label="Responsable">{selected.assigneeName ?? 'Sin asignar'}</Descriptions.Item>
          <Descriptions.Item label="Creado">{new Date(selected.createdAt).toLocaleString('es-ES')}</Descriptions.Item>
          <Descriptions.Item label="Actualizado">{new Date(selected.updatedAt).toLocaleString('es-ES')}</Descriptions.Item>
        </Descriptions>
      </Card>

      <IssueEditor
        key={selected.id}
        ticket={selected}
        onSave={async (details) => {
          try {
            await updateDetails(selected.id, details)
            message.success('Incidencia actualizada')
          } catch (error) {
            message.error(error instanceof Error ? error.message : 'No se pudo actualizar la incidencia')
          }
        }}
      />

                </div>
                <div className={styles.sideColumn}>
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

      <Card title="Asignar">
        <Space wrap>
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

      <Card title="Seguimiento">
        {watcherError && <Alert type="error" showIcon message={watcherError} />}
        <Space>
          <Button
            aria-label={watcherState?.isWatching ? 'Dejar de seguir' : 'Seguir incidencia'}
            loading={watcherLoading || watcherActionLoading}
            disabled={watcherLoading || !watcherState}
            onClick={() => void toggleWatcher()}
          >
            {watcherState?.isWatching ? 'Dejar de seguir' : 'Seguir incidencia'}
          </Button>
          <Typography.Text type="secondary">
            {watcherState?.count ?? 0} {watcherState?.count === 1 ? 'persona sigue' : 'personas siguen'} esta incidencia
          </Typography.Text>
        </Space>
      </Card>
                </div>
              </div>
            ),
          },
            {
              key: 'comments',
              label: 'Comentarios',
              children: (
                <Space direction="vertical" className={styles.collaboration}>
                  {collaborationError && <Alert type="error" showIcon message={collaborationError} />}
                  <TextArea
                    disabled={collaborationLoading}
                    value={commentBody}
                    onChange={(event) => setCommentBody(event.target.value)}
                    placeholder="Escribe un comentario. Usa @usuario para mencionar."
                    maxLength={5000}
                    rows={3}
                  />
                  <Button
                    type="primary"
                    disabled={collaborationLoading || !commentBody.trim()}
                    onClick={async () => {
                      try {
                        await createComment(selected.id, commentBody)
                        await refreshWatcherState(selected.id)
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
                    loading={collaborationLoading}
                    dataSource={collaborationLoading || commentsIssueId !== selected.id || collaborationError ? [] : comments}
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
                <Space direction="vertical" className={styles.collaboration}>
                  {collaborationError && <Alert type="error" showIcon message={collaborationError} />}
                  <List
                    loading={collaborationLoading}
                    dataSource={collaborationLoading || activitiesIssueId !== selected.id || collaborationError ? [] : activities}
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
                </Space>
              ),
            },
            {
              key: 'attachments',
              label: 'Archivos',
              children: (
                <Space direction="vertical" className={styles.collaboration}>
                  {attachmentLoadError && <Alert type="error" showIcon message={attachmentLoadError} />}
                  <Upload
                    maxCount={1}
                    disabled={attachmentsLoading}
                    showUploadList={false}
                    beforeUpload={async (file) => {
                      try {
                        const attachment = await attachmentRepository.upload(selected.id, file)
                        setAttachments((current) => [...current, attachment])
                        message.success('Archivo adjuntado')
                      } catch (error) {
                        message.error(extractErrorMessage(error, 'No se pudo adjuntar el archivo'))
                      }
                      return Upload.LIST_IGNORE
                    }}
                  >
                    <Button>Cargar archivo (máximo 10 MB)</Button>
                  </Upload>
                  <List
                    loading={attachmentsLoading}
                    dataSource={attachmentsIssueId === selected.id ? attachments : []}
                    locale={{ emptyText: 'Aún no hay archivos' }}
                    renderItem={(attachment) => (
                      <List.Item actions={[<Button key="download" onClick={async () => {
                        try {
                          await attachmentRepository.download(selected.id, attachment)
                        } catch (error) {
                          message.error(extractErrorMessage(error, 'No se pudo descargar el archivo'))
                        }
                      }}>Descargar</Button>]}>
                        <List.Item.Meta title={attachment.fileName} description={`${attachment.contentType} · ${(attachment.sizeBytes / 1024).toFixed(1)} KB`} />
                      </List.Item>
                    )}
                  />
                </Space>
              ),
            },
          ]}
      />
    </div>
  )
}

function issueTypeLabel(issueType: IssueType): string {
  const labels: Record<IssueType, string> = {
    task: 'Tarea',
    bug: 'Error',
    story: 'Historia',
    epic: 'Épica',
    subtask: 'Subtarea',
  }
  return labels[issueType]
}
