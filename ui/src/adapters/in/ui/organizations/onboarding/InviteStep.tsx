import { CopyOutlined, DeleteOutlined, PlusOutlined } from '@ant-design/icons'
import { Alert, Button, Form, Input, Select, Space, Typography } from 'antd'
import { useState } from 'react'

import { useOrgStore } from '@/adapters/in/state/org.store'
import type { InvitationCreated, OrgRole } from '@/core/organizations/domain/organization'
import { isAppError } from '@/core/shared/app-error'
import { toast } from '@/shared/ui/Toaster'
import { INVITABLE_ROLE_OPTIONS, ROLE_LABELS } from '../org-copy'
import styles from './onboarding.module.css'
import { validateInviteRows, type InviteRow, type InviteRowErrors } from './invite-rows'

export type InviteStepProps = {
  orgId: string
  onBack?: () => void
  onContinue: () => void
}

/** Wizard step 3 (optional): invitation emails with roles. No email is sent; the accept links are shown to copy. */
export function InviteStep({ orgId, onBack, onContinue }: InviteStepProps) {
  const createInvitations = useOrgStore((s) => s.createInvitations)
  const [nextId, setNextId] = useState(2)
  const [rows, setRows] = useState<InviteRow[]>([{ id: 1, email: '', role: 'member' }])
  const [rowErrors, setRowErrors] = useState<InviteRowErrors>({})
  const [submitting, setSubmitting] = useState(false)
  const [serverError, setServerError] = useState<string | null>(null)
  const [created, setCreated] = useState<InvitationCreated[] | null>(null)

  const updateRow = (id: number, patch: Partial<InviteRow>) => {
    setRows((current) => current.map((row) => (row.id === id ? { ...row, ...patch } : row)))
  }

  const removeRow = (id: number) => {
    setRows((current) => (current.length > 1 ? current.filter((row) => row.id !== id) : current))
  }

  const addRow = () => {
    setRows((current) => [...current, { id: nextId, email: '', role: 'member' }])
    setNextId((value) => value + 1)
  }

  const submit = async () => {
    setServerError(null)
    const { items, errors } = validateInviteRows(rows)
    setRowErrors(errors)
    if (Object.keys(errors).length > 0) return
    if (items.length === 0) {
      onContinue()
      return
    }
    setSubmitting(true)
    try {
      setCreated(await createInvitations(orgId, items))
    } catch (error) {
      setServerError(isAppError(error) ? error.message : 'No se pudieron crear las invitaciones. Inténtalo de nuevo.')
    } finally {
      setSubmitting(false)
    }
  }

  const copyLink = async (url: string) => {
    try {
      await navigator.clipboard.writeText(url)
      toast.success('Enlace copiado')
    } catch {
      toast.error('No se pudo copiar el enlace', 'Selecciónalo y cópialo manualmente.')
    }
  }

  if (created) {
    return (
      <div>
        <Typography.Paragraph>
          Hemos creado {created.length === 1 ? 'una invitación' : `${created.length} invitaciones`}. No enviamos correos:
          comparte cada enlace con la persona invitada. Caduca en 7 días.
        </Typography.Paragraph>
        <ul aria-label="Enlaces de invitación" className={styles.inviteList}>
          {created.map((invitation) => (
            <li key={invitation.id}>
              <Space wrap>
                <span>{invitation.email}</span>
                <span>{ROLE_LABELS[invitation.role]}</span>
                <Button
                  icon={<CopyOutlined />}
                  aria-label={`Copiar enlace de ${invitation.email}`}
                  onClick={() => void copyLink(invitation.acceptUrl)}
                >
                  Copiar enlace
                </Button>
              </Space>
            </li>
          ))}
        </ul>
        <Space className={styles.actions}>
          <Button type="primary" size="large" onClick={onContinue}>
            Continuar
          </Button>
        </Space>
      </div>
    )
  }

  return (
    <Form layout="vertical" requiredMark={false} aria-label="Invitar a tu equipo" onFinish={() => void submit()}>
      <Typography.Paragraph type="secondary">
        Añade los correos de tu equipo. Es opcional: puedes invitar a más personas después desde la configuración.
      </Typography.Paragraph>

      {rows.map((row, index) => (
        <Space key={row.id} align="start" wrap className={styles.row}>
          <Form.Item
            label={`Correo ${index + 1}`}
            validateStatus={rowErrors[row.id] ? 'error' : ''}
            help={rowErrors[row.id]}
           
          >
            <Input
              type="text"
              inputMode="email"
              autoComplete="off"
              size="large"
              value={row.email}
              aria-label={`Correo del invitado ${index + 1}`}
              onChange={(event) => updateRow(row.id, { email: event.target.value })}
            />
          </Form.Item>
          <Form.Item label="Rol">
            <Select
              size="large"
              className={styles.roleSelect}
              value={row.role}
              options={INVITABLE_ROLE_OPTIONS.map((option) => ({ ...option }))}
              aria-label={`Rol del invitado ${index + 1}`}
              onChange={(value: OrgRole) => updateRow(row.id, { role: value })}
             
            />
          </Form.Item>
          <Button
            type="text"
            icon={<DeleteOutlined />}
            aria-label={`Quitar invitado ${index + 1}`}
            disabled={rows.length === 1}
            onClick={() => removeRow(row.id)}
          />
        </Space>
      ))}

      <Button type="dashed" icon={<PlusOutlined />} onClick={addRow} className={styles.actions}>
        Añadir otro correo
      </Button>

      {serverError && <Alert type="error" showIcon message={serverError} />}

      <Space className={styles.actions}>
        {onBack && (
          <Button size="large" onClick={onBack}>
            Atrás
          </Button>
        )}
        <Button size="large" onClick={onContinue}>
          Omitir este paso
        </Button>
        <Button type="primary" size="large" htmlType="submit" loading={submitting}>
          Crear invitaciones
        </Button>
      </Space>
    </Form>
  )
}
