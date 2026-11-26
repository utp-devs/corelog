import { Button, Typography } from 'antd'

import type { InvitationCreated } from '@/core/organizations/domain/organization'
import { Section } from '@/shared/ui/Section/Section'
import { toast } from '@/shared/ui/Toaster'
import { ORG_ROLE_LABELS } from '../members/role-labels'
import { copyInviteLink } from './invite-links'
import styles from './OrgInvitationsPage.module.css'

interface InviteLinksPanelProps {
  invitations: InvitationCreated[]
  onDismiss: () => void
}

/**
 * Accept links of the invitations just created or resent. The plain token is only returned once by the
 * API, so this panel explains that the link cannot be shown again.
 */
export function InviteLinksPanel({ invitations, onDismiss }: InviteLinksPanelProps) {
  const copy = async (url: string) => {
    if (await copyInviteLink(url)) toast.success('Enlace copiado')
    else toast.error('No se pudo copiar', 'Selecciona el enlace y cópialo manualmente.')
  }

  return (
    <Section
      title="Enlaces de invitación"
      description="Este enlace solo se muestra una vez. Cópialo y compártelo ahora. Si lo pierdes, reenvía la invitación para generar uno nuevo; el anterior dejará de funcionar."
      actions={
        <Button type="text" onClick={onDismiss}>
          Ocultar
        </Button>
      }
      ariaLabel="Enlaces de invitación"
    >
      <ul className={styles.linkList}>
        {invitations.map((invitation) => (
          <li key={invitation.id} className={styles.linkItem}>
            <div className={styles.linkMeta}>
              <Typography.Text strong>{invitation.email}</Typography.Text>
              <Typography.Text type="secondary">{ORG_ROLE_LABELS[invitation.role]}</Typography.Text>
            </div>
            <Typography.Text code className={styles.linkUrl} aria-label={`Enlace de ${invitation.email}`}>
              {invitation.acceptUrl}
            </Typography.Text>
            <Button size="small" onClick={() => void copy(invitation.acceptUrl)}>
              Copiar enlace de {invitation.email}
            </Button>
          </li>
        ))}
      </ul>
    </Section>
  )
}
