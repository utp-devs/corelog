import { zodResolver } from '@hookform/resolvers/zod'
import { Alert, Button, Input, Segmented, Select, Typography } from 'antd'
import { useCallback, useEffect, useMemo, useState } from 'react'
import { Controller, useForm, useWatch } from 'react-hook-form'

import { useCurrentOrg } from '@/adapters/in/state/org.store'
import { useOrgAdminStore, type OrgAdminStore } from '@/adapters/in/state/org-admin.store'
import type { InvitationScope } from '@/core/organizations/application/ports/organization-admin-repository.port'
import { canManageOrg, type Invitation, type InvitationStatus } from '@/core/organizations/domain/organization'
import { invitableRoles } from '@/core/organizations/domain/org-permissions'
import { ConfirmModal } from '@/shared/ui/ConfirmModal'
import { DataTable, type DataTableColumn } from '@/shared/ui/DataTable'
import { EmptyState } from '@/shared/ui/EmptyState'
import { FieldRow } from '@/shared/ui/FieldRow/FieldRow'
import { Lozenge, type LozengeTone } from '@/shared/ui/Lozenge'
import { PageContainer } from '@/shared/ui/PageContainer'
import { PageHeader } from '@/shared/ui/PageHeader'
import { Section } from '@/shared/ui/Section/Section'
import { toast } from '@/shared/ui/Toaster'
import { adminErrorMessage } from '../members/member-messages'
import { formatOrgDate, ORG_ROLE_LABELS } from '../members/role-labels'
import { InviteLinksPanel } from './InviteLinksPanel'
import { inviteFormSchema, inviteItemsOf, MAX_INVITATIONS_PER_REQUEST, parseInviteEmails, type InviteFormValues } from './invite-form'
import styles from './OrgInvitationsPage.module.css'

export interface OrgInvitationsPageProps {
  /** Injected for tests; the app uses the application singleton. */
  store?: OrgAdminStore
}

const STATUS_LABELS: Record<InvitationStatus, string> = {
  pending: 'Pendiente',
  accepted: 'Aceptada',
  revoked: 'Revocada',
  expired: 'Caducada',
}

const STATUS_TONES: Record<InvitationStatus, LozengeTone> = {
  pending: 'info',
  accepted: 'success',
  revoked: 'danger',
  expired: 'warning',
}

const SCOPE_OPTIONS = [
  { label: 'Pendientes', value: 'pending' },
  { label: 'Todas', value: 'all' },
]

/** Moves the focus to the email field of the invitation form (empty-state call to action). */
function focusInviteField() {
  document.querySelector<HTMLTextAreaElement>('form[aria-label="Invitar personas"] textarea')?.focus()
}

/** Invitations of the active organization (S-22). The plain link is shown once, right after creation or resend. */
export function OrgInvitationsPage({ store = useOrgAdminStore }: OrgInvitationsPageProps) {
  const { org, role } = useCurrentOrg()
  const canManage = canManageOrg(role)
  const orgId = org?.id ?? null
  const invitations = store((s) => s.invitations)
  const scope = store((s) => s.invitationScope)
  const status = store((s) => s.invitationsStatus)
  const loadError = store((s) => s.invitationsError)
  const members = store((s) => s.members)
  const created = store((s) => s.createdInvitations)

  const [submitError, setSubmitError] = useState<string | null>(null)
  const [revoking, setRevoking] = useState<Invitation | null>(null)
  const [busy, setBusy] = useState(false)

  const roleOptions = useMemo(
    () => invitableRoles(role).map((item) => ({ value: item, label: ORG_ROLE_LABELS[item] })),
    [role],
  )

  const {
    control,
    handleSubmit,
    reset,
    formState: { errors, isSubmitting },
  } = useForm<InviteFormValues>({
    resolver: zodResolver(inviteFormSchema),
    defaultValues: { emails: '', role: 'member' },
    disabled: !canManage,
  })

  const emailsText = useWatch({ control, name: 'emails' })
  const parsed = useMemo(() => parseInviteEmails(emailsText ?? ''), [emailsText])

  const load = useCallback(
    (next?: InvitationScope) => {
      if (!orgId || !canManage) return
      void store.getState().fetchInvitations(orgId, next)
      void store.getState().fetchMembers(orgId)
    },
    [store, orgId, canManage],
  )

  useEffect(() => {
    load()
  }, [load])

  const onSubmit = handleSubmit(async (values) => {
    if (!orgId) return
    setSubmitError(null)
    try {
      const items = inviteItemsOf(values)
      await store.getState().createInvitations(orgId, items)
      reset({ emails: '', role: values.role })
      toast.success(items.length === 1 ? 'Invitación creada' : `${items.length} invitaciones creadas`)
    } catch (error) {
      setSubmitError(adminErrorMessage(error, 'No se pudieron enviar las invitaciones.'))
    }
  })

  const resend = async (invitation: Invitation) => {
    if (!orgId) return
    try {
      await store.getState().resendInvitation(orgId, invitation.id)
      toast.success('Nuevo enlace generado. El anterior ya no funciona.')
    } catch (error) {
      toast.error(adminErrorMessage(error, 'No se pudo reenviar la invitación.'))
    }
  }

  const revoke = async () => {
    if (!revoking || !orgId) return
    setBusy(true)
    try {
      await store.getState().revokeInvitation(orgId, revoking.id)
      toast.success('Invitación revocada')
    } catch (error) {
      toast.error(adminErrorMessage(error, 'No se pudo revocar la invitación.'))
    } finally {
      setBusy(false)
      setRevoking(null)
    }
  }

  const invitedByName = (userId: string | null): string => {
    if (!userId) return '—'
    return members.find((member) => member.userId === userId)?.name ?? '—'
  }

  const columns: DataTableColumn<Invitation>[] = [
    {
      key: 'email',
      title: 'Correo',
      mobilePrimary: true,
      render: (_value, invitation) => <span className={styles.email}>{invitation.email}</span>,
    },
    {
      key: 'role',
      title: 'Rol',
      render: (_value, invitation) => <Lozenge tone="neutral">{ORG_ROLE_LABELS[invitation.role]}</Lozenge>,
    },
    {
      key: 'status',
      title: 'Estado',
      mobilePrimary: true,
      render: (_value, invitation) => (
        <Lozenge tone={STATUS_TONES[invitation.status]}>{STATUS_LABELS[invitation.status]}</Lozenge>
      ),
    },
    {
      key: 'expiresAt',
      title: 'Caduca',
      render: (_value, invitation) =>
        invitation.status === 'pending' ? formatOrgDate(invitation.expiresAt) : '—',
    },
    {
      key: 'invitedBy',
      title: 'Invitada por',
      render: (_value, invitation) => invitedByName(invitation.invitedBy),
    },
  ]

  const pendingOnly = (invitation: Invitation) => invitation.status === 'pending'

  const emptyState = (
    <EmptyState
      title={scope === 'pending' ? 'No hay invitaciones pendientes' : 'Aún no has invitado a nadie'}
      description="Escribe uno o varios correos arriba y elige su rol para enviar las invitaciones."
      action={
        canManage ? (
          <Button onClick={focusInviteField}>Invitar personas</Button>
        ) : undefined
      }
    />
  )

  if (org && !canManage) {
    return (
      <PageContainer variant="wide" pageName="org-invitations" ready>
        <PageHeader title="Invitaciones" description={org.name} />
        <Alert
          type="info"
          showIcon
          className={styles.banner}
          message="Solo lectura"
          description="Solo los propietarios y administradores pueden invitar personas y gestionar las invitaciones."
        />
        <EmptyState
          title="No tienes permiso para ver esto"
          description="Pide acceso a un administrador de la organización."
        />
      </PageContainer>
    )
  }

  const loading = orgId !== null && (status === 'idle' || status === 'loading')
  const error = orgId && status === 'error' ? (loadError?.message ?? 'No se pudieron cargar las invitaciones.') : null

  return (
    <PageContainer variant="wide" pageName="org-invitations" ready={!loading}>
      <PageHeader title="Invitaciones" description={org ? `Personas invitadas a ${org.name}` : undefined} />

      <Section title="Invitar personas" description={`Puedes escribir hasta ${MAX_INVITATIONS_PER_REQUEST} correos separados por comas o saltos de línea.`}>
        <form aria-label="Invitar personas" onSubmit={onSubmit} noValidate className={styles.form}>
          <FieldRow
            label="Correos electrónicos"
            required
            error={errors.emails?.message ?? (parsed.invalid.length > 0 ? `No válidos: ${parsed.invalid.join(', ')}` : undefined)}
            helper={
              parsed.duplicates.length > 0
                ? `Se omitirán los repetidos: ${parsed.duplicates.join(', ')}`
                : `${parsed.emails.length} correo(s) listo(s) para invitar`
            }
          >
            {(props) => (
              <Controller
                name="emails"
                control={control}
                render={({ field }) => (
                  <Input.TextArea
                    {...field}
                    {...props}
                    rows={3}
                    placeholder="ana@empresa.com, luis@empresa.com"
                    disabled={!canManage}
                  />
                )}
              />
            )}
          </FieldRow>
          <FieldRow label="Rol para todos" required>
            {(props) => (
              <Controller
                name="role"
                control={control}
                render={({ field }) => (
                  <Select {...field} {...props} options={roleOptions} className={styles.roleSelect} disabled={!canManage} />
                )}
              />
            )}
          </FieldRow>
          {submitError && (
            <Alert type="error" showIcon role="alert" message={submitError} />
          )}
          <div className={styles.formActions}>
            <Button type="primary" htmlType="submit" loading={isSubmitting} disabled={!canManage}>
              Enviar invitaciones
            </Button>
          </div>
        </form>
      </Section>

      {created.length > 0 && (
        <InviteLinksPanel invitations={created} onDismiss={() => store.getState().clearCreatedInvitations()} />
      )}

      <div className={styles.toolbar}>
        <Segmented
          aria-label="Mostrar invitaciones"
          options={SCOPE_OPTIONS}
          value={scope}
          onChange={(value) => load(value as InvitationScope)}
        />
      </div>

      <DataTable<Invitation>
        tableKey="org-invitations"
        columns={columns}
        data={orgId ? invitations : []}
        rowKey="id"
        loading={loading}
        error={error}
        onRetry={() => load()}
        emptyState={emptyState}
        rowActions={(invitation) =>
          pendingOnly(invitation)
            ? [
                { key: 'resend', label: 'Reenviar (nuevo enlace)', onClick: () => void resend(invitation) },
                { key: 'revoke', label: 'Revocar', danger: true, onClick: () => setRevoking(invitation) },
              ]
            : []
        }
        mobileRender={(invitation) => (
          <div className={styles.mobileCard}>
            <span className={styles.email}>{invitation.email}</span>
            <Lozenge tone={STATUS_TONES[invitation.status]}>{STATUS_LABELS[invitation.status]}</Lozenge>
            <Typography.Text type="secondary">{ORG_ROLE_LABELS[invitation.role]}</Typography.Text>
          </div>
        )}
      />

      <ConfirmModal
        open={revoking !== null}
        title="¿Revocar la invitación?"
        description={revoking ? `El enlace enviado a ${revoking.email} dejará de funcionar de inmediato.` : undefined}
        confirmText="Revocar"
        danger
        loading={busy}
        onConfirm={revoke}
        onCancel={() => setRevoking(null)}
      />
    </PageContainer>
  )
}
