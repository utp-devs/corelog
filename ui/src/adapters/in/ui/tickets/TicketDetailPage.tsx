import { ArrowLeftOutlined, EditOutlined } from '@ant-design/icons'
import { Alert, App, Button, DatePicker, Input, InputNumber, Select, Upload } from 'antd'
import dayjs, { type Dayjs } from 'dayjs'
import { useEffect, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'

import { extractErrorMessage } from '@/adapters/out/http/api-client'
import { AttachmentHttpRepository } from '@/adapters/out/http/attachment-http-repository'
import { IssueWatcherHttpRepository } from '@/adapters/out/http/issue-watcher-http-repository'
import { useTicketsStore } from '@/adapters/in/state/tickets.store'
import type { UpdateIssueDetailsInput } from '@/core/tickets/application/ports/ticket-repository.port'
import type { IssueAttachment } from '@/core/tickets/domain/attachment'
import type { IssueWatcherState } from '@/core/tickets/domain/issue-watcher'
import { canTransitionTo, type IssueType, type Ticket, type TicketPriority, type TicketStatus } from '@/core/tickets/domain/ticket'
import { useTrackRecent } from '@/shared/ui/CommandPalette/recent'
import { EmptyState } from '@/shared/ui/EmptyState'
import { ErrorState } from '@/shared/ui/ErrorState'
import { FieldRow } from '@/shared/ui/FieldRow'
import { InlineEdit } from '@/shared/ui/InlineEdit'
import { IssueKey } from '@/shared/ui/IssueKey'
import { LoadingSkeleton } from '@/shared/ui/LoadingSkeleton'
import { PageContainer } from '@/shared/ui/PageContainer'
import { RichTextEditor } from '@/shared/ui/RichTextEditor'
import { RichTextView } from '@/shared/ui/RichTextView'
import { Section } from '@/shared/ui/Section'
import { IssuePicker } from './IssuePicker'
import { useMemberNames } from './hooks/useMemberNames'
import { useTeammateSearch } from './hooks/useTeammateSearch'
import { issueKeyOf, issueTypeLabels, priorityLabels, statusMeta } from './ticketLabels'
import { searchMentionUsers } from './mentions'
import styles from './TicketDetailPage.module.css'

const attachmentRepository = new AttachmentHttpRepository()
const watcherRepository = new IssueWatcherHttpRepository()

const allStatuses: TicketStatus[] = ['open', 'in_progress', 'resolved', 'closed']
const issueTypeOptions = (Object.keys(issueTypeLabels) as IssueType[]).map((value) => ({ value, label: issueTypeLabels[value] }))
const priorityOptions = (Object.keys(priorityLabels) as TicketPriority[]).map((value) => ({ value, label: priorityLabels[value] }))

// Etiquetas en español para el tipo de vinculo entre incidencias.
const linkKindLabels: Record<string, string> = {
  blocks: 'Bloquea a',
  relates: 'Se relaciona con',
  duplicates: 'Duplica a',
}

const linkKindOptions = [
  { value: 'blocks', label: 'Bloquea' },
  { value: 'relates', label: 'Se relaciona con' },
  { value: 'duplicates', label: 'Duplica' },
]

/** Copia de la incidencia con los campos de detalle, sobrescribiendo solo los indicados. */
function payloadFrom(ticket: Ticket, overrides: Partial<UpdateIssueDetailsInput> = {}): UpdateIssueDetailsInput {
  return {
    title: ticket.title,
    description: ticket.description,
    priority: ticket.priority,
    issueType: ticket.issueType,
    parentId: ticket.parentId ?? undefined,
    dueDate: ticket.dueDate ?? undefined,
    storyPoints: ticket.storyPoints ?? undefined,
    ...overrides,
  }
}

function formatDateTime(value: string): string {
  return dayjs(value).format('DD/MM/YYYY HH:mm')
}

type DescriptionSectionProps = {
  ticket: Ticket
  onSave: (input: UpdateIssueDetailsInput) => Promise<boolean>
}

/** Descripción en vista (Markdown sanitizado) y edición con los datos de detalle. */
function DescriptionSection({ ticket, onSave }: DescriptionSectionProps) {
  const [editing, setEditing] = useState(false)
  const [saving, setSaving] = useState(false)
  const [description, setDescription] = useState(ticket.description)
  const [issueType, setIssueType] = useState<IssueType>(ticket.issueType)
  const [parentId, setParentId] = useState(ticket.parentId ?? '')
  const [dueDate, setDueDate] = useState<Dayjs | null>(ticket.dueDate ? dayjs(ticket.dueDate) : null)
  const [storyPoints, setStoryPoints] = useState<number | null>(ticket.storyPoints)

  const startEdit = () => {
    setDescription(ticket.description)
    setIssueType(ticket.issueType)
    setParentId(ticket.parentId ?? '')
    setDueDate(ticket.dueDate ? dayjs(ticket.dueDate) : null)
    setStoryPoints(ticket.storyPoints)
    setEditing(true)
  }

  const save = async () => {
    setSaving(true)
    const saved = await onSave(payloadFrom(ticket, {
      description,
      issueType,
      parentId: parentId || undefined,
      dueDate: dueDate?.toISOString(),
      storyPoints: storyPoints ?? undefined,
    }))
    setSaving(false)
    if (saved) setEditing(false)
  }

  return (
    <Section
      title="Descripción"
      actions={editing ? undefined : <Button icon={<EditOutlined />} onClick={startEdit}>Editar</Button>}
    >
      {editing ? (
        <form
          className={styles.editForm}
          aria-label="Editar descripción y datos"
          onSubmit={(event) => {
            event.preventDefault()
            void save()
          }}
        >
          <RichTextEditor
            ariaLabel="Descripción"
            value={description}
            onChange={setDescription}
            minRows={6}
            searchUsers={searchMentionUsers}
            disabled={saving}
          />
          <div className={styles.fieldGrid}>
            <FieldRow label="Tipo">
              {({ id }) => (
                <Select<IssueType> id={id} value={issueType} onChange={setIssueType} options={issueTypeOptions} />
              )}
            </FieldRow>
            <FieldRow label="Fecha límite">
              {({ id }) => <DatePicker id={id} value={dueDate} onChange={setDueDate} />}
            </FieldRow>
            <FieldRow label="Puntos de historia">
              {({ id }) => <InputNumber id={id} value={storyPoints} onChange={setStoryPoints} min={0} max={999} />}
            </FieldRow>
            <FieldRow label="Incidencia padre" helper="Identificador de la incidencia padre, si la hay.">
              {({ id, 'aria-describedby': describedBy }) => (
                <Input id={id} aria-describedby={describedBy} value={parentId} onChange={(event) => setParentId(event.target.value)} />
              )}
            </FieldRow>
          </div>
          <div className={styles.actions}>
            <Button onClick={() => setEditing(false)} disabled={saving}>Cancelar</Button>
            <Button type="primary" htmlType="submit" loading={saving}>Guardar</Button>
          </div>
        </form>
      ) : ticket.description.trim() ? (
        <RichTextView value={ticket.description} />
      ) : (
        <EmptyState title="Sin descripción" description="Añade una descripción para dar contexto a la incidencia." />
      )}
    </Section>
  )
}

function DetailsSection({ ticket }: { ticket: Ticket }) {
  return (
    <Section title="Detalles">
      <dl className={styles.details}>
        <dt>Tipo</dt>
        <dd>{issueTypeLabels[ticket.issueType]}</dd>
        <dt>Fecha límite</dt>
        <dd>{ticket.dueDate ? dayjs(ticket.dueDate).format('DD/MM/YYYY') : 'Sin fecha límite'}</dd>
        <dt>Puntos de historia</dt>
        <dd>{ticket.storyPoints ?? 'Sin estimar'}</dd>
        <dt>Incidencia padre</dt>
        <dd>{ticket.parentId ?? 'Sin incidencia padre'}</dd>
        <dt>Solicitante</dt>
        <dd>{ticket.requesterName}</dd>
        <dt>Creado</dt>
        <dd>{formatDateTime(ticket.createdAt)}</dd>
        <dt>Actualizado</dt>
        <dd>{formatDateTime(ticket.updatedAt)}</dd>
      </dl>
    </Section>
  )
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
  // Recents: the issue shown here, once its ticket for this id has loaded.
  useTrackRecent(selected && selected.id === id
    ? { kind: 'issue', id: selected.id, title: selected.title, subtitle: 'Incidencia', link: `/issues/${selected.id}` }
    : null)

  const [assigning, setAssigning] = useState(false)
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
  const { nameOf } = useMemberNames()

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
  }, [id, fetchComments, fetchActivities, fetchLinks])

  if (status === 'loading' || (!selected && status === 'idle')) {
    return (
      <PageContainer variant="wide" pageName="issue-detail" ready={false}>
        <LoadingSkeleton variant="detail" />
      </PageContainer>
    )
  }

  if (status === 'error' || !selected || selected.id !== id) {
    return (
      <PageContainer variant="wide" pageName="issue-detail" ready>
        <ErrorState
          message={ticketError ?? 'No se pudo encontrar la incidencia solicitada.'}
          onRetry={id ? () => void fetchTicket(id) : undefined}
        />
        <div className={styles.backRow}>
          <Button onClick={() => navigate('/issues')}>Volver a incidencias</Button>
        </div>
      </PageContainer>
    )
  }

  // The current assignee stays selectable even when the last search did not return it.
  const assigneeOptions = [
    ...(selected.assigneeId && !teammateSearch.results.some((u) => u.id === selected.assigneeId)
      ? [{ value: selected.assigneeId, label: selected.assigneeName ?? selected.assigneeId }]
      : []),
    ...teammateSearch.results.map((u) => ({ value: u.id, label: `${u.name} (${u.email})` })),
  ]

  const nextStatuses = allStatuses.filter((s) => canTransitionTo(selected.status, s))
  const statusOptions = allStatuses
    .filter((s) => s === selected.status || nextStatuses.includes(s))
    .map((s) => ({ value: s, label: statusMeta[s].label }))

  const refreshWatcherState = async (issueId: string) => {
    try {
      const state = await watcherRepository.getState(issueId)
      setWatcherResult({ issueId, state })
    } catch (error) {
      setWatcherResult({ issueId, error: extractErrorMessage(error, 'No se pudo actualizar el seguimiento') })
    }
  }

  const saveTitle = async (next: string | null) => {
    const title = next?.trim()
    if (!title) throw new Error('El título es obligatorio')
    await updateDetails(selected.id, payloadFrom(selected, { title }))
    message.success('Título actualizado')
  }

  const saveStatus = async (next: string | null) => {
    if (!next) throw new Error('El estado es obligatorio')
    await updateStatus(selected.id, next as TicketStatus)
    message.success('Estado actualizado')
  }

  const savePriority = async (next: string | null) => {
    if (!next) throw new Error('La prioridad es obligatoria')
    await updateDetails(selected.id, payloadFrom(selected, { priority: next as TicketPriority }))
    message.success('Prioridad actualizada')
  }

  const saveDetails = async (input: UpdateIssueDetailsInput): Promise<boolean> => {
    try {
      await updateDetails(selected.id, input)
      message.success('Incidencia actualizada')
      return true
    } catch (error) {
      message.error(extractErrorMessage(error, 'No se pudo actualizar la incidencia'))
      return false
    }
  }

  // Assigning saves on selection, like status and priority. The API has no way to clear the assignee yet.
  const handleAssign = async (assigneeId: string) => {
    setAssigning(true)
    try {
      await assign(selected.id, assigneeId)
      await refreshWatcherState(selected.id)
      message.success('Incidencia asignada')
    } catch (error) {
      message.error(extractErrorMessage(error, 'No se pudo asignar la incidencia'))
    } finally {
      setAssigning(false)
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

  const createLinkNow = async () => {
    try {
      await createLink(selected.id, linkTargetId, linkKind)
      setLinkTargetId('')
      message.success('Vinculo creado')
    } catch (error) {
      message.error(extractErrorMessage(error, 'No se pudo crear el vinculo'))
    }
  }

  const submitComment = async () => {
    if (!commentBody.trim()) return
    try {
      await createComment(selected.id, commentBody)
      await refreshWatcherState(selected.id)
      setCommentBody('')
      message.success('Comentario agregado')
    } catch (error) {
      message.error(extractErrorMessage(error, 'No se pudo agregar el comentario'))
    }
  }

  const removeComment = async (commentId: string) => {
    try {
      await deleteComment(selected.id, commentId)
      message.success('Comentario eliminado')
    } catch (error) {
      message.error(extractErrorMessage(error, 'No se pudo eliminar'))
    }
  }

  const uploadAttachment = async (file: File) => {
    try {
      const attachment = await attachmentRepository.upload(selected.id, file)
      setAttachments((current) => [...current, attachment])
      message.success('Archivo adjuntado')
    } catch (error) {
      message.error(extractErrorMessage(error, 'No se pudo adjuntar el archivo'))
    }
    return Upload.LIST_IGNORE
  }

  const downloadAttachment = async (attachment: IssueAttachment) => {
    try {
      await attachmentRepository.download(selected.id, attachment)
    } catch (error) {
      message.error(extractErrorMessage(error, 'No se pudo descargar el archivo'))
    }
  }

  const visibleLinks = collaborationLoading || linksIssueId !== selected.id || collaborationError ? [] : links
  const visibleComments = collaborationLoading || commentsIssueId !== selected.id || collaborationError ? [] : comments
  const visibleActivities = collaborationLoading || activitiesIssueId !== selected.id || collaborationError ? [] : activities
  const visibleAttachments = attachmentsLoading || attachmentLoadError ? [] : attachments

  const main = (
    <>
      {collaborationError && <Alert type="error" showIcon message={collaborationError} />}

      <header className={styles.header}>
        <Button type="text" icon={<ArrowLeftOutlined />} className={styles.back} onClick={() => navigate('/issues')}>
          Volver a incidencias
        </Button>
        <IssueKey
          issueKey={issueKeyOf(selected.id)}
          href={`/issues/${selected.id}`}
          summary={selected.title}
          type={selected.issueType}
          resolved={selected.status === 'resolved' || selected.status === 'closed'}
        />
        <h1 className={styles.title}>
          <InlineEdit label="Título" type="text" value={selected.title} onSave={saveTitle} />
        </h1>
      </header>

      <DescriptionSection key={selected.id} ticket={selected} onSave={saveDetails} />
      <DetailsSection ticket={selected} />

      <Section title="Vínculos">
        <div className={styles.linkForm}>
          <IssuePicker value={linkTargetId} onChange={setLinkTargetId} excludeId={selected.id} />
          <Select
            aria-label="Tipo de vínculo"
            value={linkKind}
            onChange={setLinkKind}
            options={linkKindOptions}
          />
          <Button
            type="primary"
            disabled={collaborationLoading || !linkTargetId}
            onClick={() => void createLinkNow()}
          >
            Vincular
          </Button>
        </div>
        {collaborationLoading ? (
          <LoadingSkeleton variant="card" />
        ) : visibleLinks.length === 0 && !collaborationError ? (
          <EmptyState title="Aún no hay vínculos" />
        ) : (
          <ul className={styles.list}>
            {visibleLinks.map((link) => {
              const otherId = link.sourceIssueId === selected.id ? link.targetIssueId : link.sourceIssueId
              return (
                <li key={link.id} className={styles.listItem}>
                  <Link to={`/issues/${otherId}`}>
                    {linkKindLabels[link.kind] ?? link.kind}: incidencia {otherId.slice(0, 8)}
                  </Link>
                  <Button danger size="small" onClick={() => void deleteLink(selected.id, link.id)}>
                    Eliminar
                  </Button>
                </li>
              )
            })}
          </ul>
        )}
      </Section>

      <Section title="Adjuntos">
        {attachmentLoadError && <Alert type="error" showIcon message={attachmentLoadError} />}
        <Upload maxCount={1} disabled={attachmentsLoading} showUploadList={false} beforeUpload={uploadAttachment}>
          <Button>Cargar archivo (máximo 10 MB)</Button>
        </Upload>
        {attachmentsLoading ? (
          <LoadingSkeleton variant="card" />
        ) : visibleAttachments.length === 0 && !attachmentLoadError ? (
          <EmptyState title="Aún no hay archivos" />
        ) : (
          <ul className={styles.list}>
            {visibleAttachments.map((attachment) => (
              <li key={attachment.id} className={styles.listItem}>
                <span className={styles.fileInfo}>
                  <span className={styles.fileName}>{attachment.fileName}</span>
                  <span className={styles.muted}>
                    {attachment.contentType} · {(attachment.sizeBytes / 1024).toFixed(1)} KB · Subido por {nameOf(attachment.uploaderId)}
                  </span>
                </span>
                <Button onClick={() => void downloadAttachment(attachment)}>Descargar</Button>
              </li>
            ))}
          </ul>
        )}
      </Section>

      <Section title="Comentarios">
        <RichTextEditor
          ariaLabel="Nuevo comentario"
          value={commentBody}
          onChange={setCommentBody}
          onSubmit={() => void submitComment()}
          placeholder="Escribe un comentario. Usa @ para mencionar."
          minRows={3}
          searchUsers={searchMentionUsers}
          disabled={collaborationLoading}
        />
        <div className={styles.actions}>
          <Button
            type="primary"
            disabled={collaborationLoading || !commentBody.trim()}
            onClick={() => void submitComment()}
          >
            Comentar
          </Button>
        </div>
        {collaborationLoading ? (
          <LoadingSkeleton variant="card" />
        ) : visibleComments.length === 0 && !collaborationError ? (
          <EmptyState title="Aún no hay comentarios" />
        ) : (
          <ol className={styles.list}>
            {visibleComments.map((comment) => (
              <li key={comment.id} className={styles.comment}>
                <div className={styles.commentHeader}>
                  <span className={styles.author}>{nameOf(comment.authorId)}</span>
                  <time dateTime={comment.createdAt} className={styles.muted}>{formatDateTime(comment.createdAt)}</time>
                  <Button danger size="small" onClick={() => void removeComment(comment.id)}>
                    Eliminar
                  </Button>
                </div>
                <RichTextView value={comment.body} />
              </li>
            ))}
          </ol>
        )}
      </Section>

      <Section title="Historial">
        {collaborationLoading ? (
          <LoadingSkeleton variant="card" />
        ) : visibleActivities.length === 0 && !collaborationError ? (
          <EmptyState title="Aún no hay actividad" />
        ) : (
          <ol className={styles.list}>
            {visibleActivities.map((activity) => (
              <li key={activity.id} className={styles.activity}>
                <span>
                  {activity.action}
                  {activity.fieldName ? ` - ${activity.fieldName}` : ''}
                  <span className={styles.muted}> · {nameOf(activity.actorId)}</span>
                </span>
                <time dateTime={activity.createdAt} className={styles.muted}>{formatDateTime(activity.createdAt)}</time>
              </li>
            ))}
          </ol>
        )}
      </Section>
    </>
  )

  const sidebar = (
    <>
      <Section title="Estado y prioridad">
        <dl className={styles.details}>
          <dt>Estado</dt>
          <dd>
            <InlineEdit label="Estado" type="select" value={selected.status} options={statusOptions} onSave={saveStatus} />
          </dd>
          <dt>Prioridad</dt>
          <dd>
            <InlineEdit label="Prioridad" type="select" value={selected.priority} options={priorityOptions} onSave={savePriority} />
          </dd>
        </dl>
      </Section>

      <Section title="Responsable">
        <FieldRow label="Buscar compañero" helper={`Actual: ${selected.assigneeName ?? 'Sin asignar'}`}>
          {({ id, 'aria-describedby': describedBy }) => (
            <Select
              id={id}
              aria-describedby={describedBy}
              showSearch
              filterOption={false}
              placeholder="Nombre o correo"
              loading={teammateSearch.isLoading || assigning}
              disabled={assigning}
              notFoundContent={teammateSearch.isLoading ? 'Buscando...' : 'No se encontraron compañeros'}
              onSearch={teammateSearch.search}
              onChange={(value: string) => void handleAssign(value)}
              value={selected.assigneeId ?? undefined}
              options={assigneeOptions}
            />
          )}
        </FieldRow>
      </Section>

      <Section title="Seguimiento">
        {watcherError && <Alert type="error" showIcon message={watcherError} />}
        <div className={styles.inlineRow}>
          <Button
            aria-label={watcherState?.isWatching ? 'Dejar de seguir' : 'Seguir incidencia'}
            loading={watcherLoading || watcherActionLoading}
            disabled={watcherLoading || !watcherState}
            onClick={() => void toggleWatcher()}
          >
            {watcherState?.isWatching ? 'Dejar de seguir' : 'Seguir incidencia'}
          </Button>
        </div>
        <p className={styles.muted}>
          {watcherState?.count ?? 0} {watcherState?.count === 1 ? 'persona sigue' : 'personas siguen'} esta incidencia
        </p>
      </Section>
    </>
  )

  return (
    <PageContainer variant="wide" pageName="issue-detail" ready sidebar={sidebar}>
      {main}
    </PageContainer>
  )
}
