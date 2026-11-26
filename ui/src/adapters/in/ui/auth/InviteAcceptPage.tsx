import { Alert, App, Button, Spin, Typography } from 'antd'
import { useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'

import { useAuthStore } from '@/adapters/in/state/auth.store'
import { useOrgStore } from '@/adapters/in/state/org.store'
import type { InvitationPreview } from '@/core/organizations/domain/organization'
import { isAppError } from '@/core/shared/app-error'
import { invitationErrorMessage, ROLE_LABELS } from '../organizations/org-copy'
import { AuthLayout } from './AuthLayout'
import styles from './AuthPage.module.css'
import { authPathWithRedirect } from './safe-redirect'

type PreviewState =
  | { kind: 'loading' }
  | { kind: 'invalid' }
  | { kind: 'failed'; message: string }
  | { kind: 'ready'; preview: InvitationPreview }

const UNUSABLE_MESSAGES: Record<string, string> = {
  accepted: 'Esta invitación ya se ha utilizado. Si ya eres miembro, entra desde tu organización.',
  revoked: 'Esta invitación fue revocada. Pide a un administrador que te envíe una nueva.',
  expired: 'La invitación ha caducado. Pide a un administrador un enlace nuevo.',
}

/** Public invitation page (S-04): previews the invitation and lets the signed-in invitee accept it. */
export function InviteAcceptPage() {
  const { token = '' } = useParams()
  const navigate = useNavigate()
  const { message } = App.useApp()
  const sessionChecked = useAuthStore((s) => s.sessionChecked)
  const user = useAuthStore((s) => s.user)
  const logout = useAuthStore((s) => s.logout)
  const previewInvitation = useOrgStore((s) => s.previewInvitation)
  const acceptInvitation = useOrgStore((s) => s.acceptInvitation)
  const fetchOrgs = useOrgStore((s) => s.fetchOrgs)
  const setCurrentOrg = useOrgStore((s) => s.setCurrentOrg)

  const [state, setState] = useState<PreviewState>({ kind: 'loading' })
  const [accepting, setAccepting] = useState(false)
  const [acceptError, setAcceptError] = useState<string | null>(null)
  const returnPath = `/invite/${encodeURIComponent(token)}`

  useEffect(() => {
    let active = true
    previewInvitation(token)
      .then((preview) => {
        if (active) setState({ kind: 'ready', preview })
      })
      .catch((error: unknown) => {
        if (!active) return
        if (isAppError(error) && error.status === 404) {
          setState({ kind: 'invalid' })
        } else {
          setState({ kind: 'failed', message: isAppError(error) ? error.message : 'No se pudo cargar la invitación.' })
        }
      })
    return () => {
      active = false
    }
  }, [token, previewInvitation])

  const onAccept = async (preview: InvitationPreview) => {
    setAccepting(true)
    setAcceptError(null)
    try {
      const membership = await acceptInvitation(token)
      await fetchOrgs()
      setCurrentOrg(membership.organizationId)
      message.success(`Te has unido a ${preview.organizationName}`)
      navigate('/projects', { replace: true })
    } catch (error) {
      setAcceptError(invitationErrorMessage(error, preview.email))
    } finally {
      setAccepting(false)
    }
  }

  const switchAccount = () => {
    logout()
    navigate(authPathWithRedirect('/login', returnPath), { replace: true })
  }

  if (!sessionChecked || state.kind === 'loading') {
    return (
      <AuthLayout title="Invitación" subtitle="Comprobando la invitación..." footer={null}>
        <div className="route-loading">
          <Spin size="large" />
        </div>
      </AuthLayout>
    )
  }

  if (state.kind === 'invalid' || state.kind === 'failed') {
    return (
      <AuthLayout title="Invitación no disponible" subtitle="No pudimos abrir esta invitación." footer={null}>
        <Alert
          type="error"
          showIcon
          message={state.kind === 'invalid' ? 'Este enlace no es válido.' : state.message}
          description="Revisa el enlace o pide a un administrador que te envíe una invitación nueva."
        />
      </AuthLayout>
    )
  }

  const { preview } = state
  if (preview.status !== 'pending') {
    return (
      <AuthLayout title={`Invitación a ${preview.organizationName}`} subtitle="Esta invitación ya no se puede aceptar." footer={null}>
        <Alert type="warning" showIcon message={UNUSABLE_MESSAGES[preview.status] ?? 'Esta invitación no está disponible.'} />
      </AuthLayout>
    )
  }

  const signedIn = user !== null
  const sameAccount = signedIn && user.email.toLowerCase() === preview.email.toLowerCase()

  return (
    <AuthLayout
      title={`Invitación a ${preview.organizationName}`}
      subtitle={`Te invitan a unirte como ${ROLE_LABELS[preview.role]}.`}
      footer={null}
    >
      <Typography.Paragraph className={styles.form}>
        <strong>Organización:</strong> {preview.organizationName}
        <br />
        <strong>Rol:</strong> {ROLE_LABELS[preview.role]}
        <br />
        <strong>Correo invitado:</strong> {preview.email}
      </Typography.Paragraph>

      {!signedIn && (
        <>
          <Typography.Paragraph type="secondary">
            Inicia sesión o crea una cuenta con {preview.email} para aceptar la invitación.
          </Typography.Paragraph>
          <div className={styles.form}>
            <Button type="primary" block size="large" onClick={() => navigate(authPathWithRedirect('/login', returnPath))}>
              Iniciar sesión
            </Button>
            <Button block size="large" onClick={() => navigate(authPathWithRedirect('/register', returnPath))}>
              Crear cuenta
            </Button>
          </div>
        </>
      )}

      {signedIn && !sameAccount && (
        <Alert
          type="warning"
          showIcon
          className={styles.form}
          message={`Esta invitación es para ${preview.email}, pero has iniciado sesión como ${user.email}.`}
          description="Cambia de cuenta para aceptarla."
        />
      )}

      {signedIn && (
        <>
          {acceptError && <Alert type="error" showIcon className={styles.form} message={acceptError} />}
          <div className={styles.form}>
            <Button
              type="primary"
              block
              size="large"
              loading={accepting}
              onClick={() => void onAccept(preview)}
            >
              Aceptar invitación
            </Button>
            {(!sameAccount || acceptError) && (
              <Button block size="large" onClick={switchAccount}>
                Cambiar de cuenta
              </Button>
            )}
          </div>
        </>
      )}
    </AuthLayout>
  )
}
