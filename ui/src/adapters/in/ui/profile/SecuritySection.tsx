import { CheckCircleFilled, DeleteOutlined } from '@ant-design/icons'
import { Alert, App, Button, Card, Input, Modal, Popconfirm, Skeleton, Space, Switch, Tag, Typography } from 'antd'
import dayjs from 'dayjs'
import { useCallback, useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'

import { useAuthStore } from '@/adapters/in/state/auth.store'
import { extractErrorMessage } from '@/adapters/out/http/api-client'
import { AccountSecurityHttpRepository } from '@/adapters/out/http/account-security-http-repository'
import type { ChangePasswordPayload } from '@/core/auth/application/ports/auth-repository.port'
import { describeUserAgent, type AccountSession } from '@/core/auth/domain/session'
import { PasswordForm } from './PasswordForm'
import styles from './ProfilePage.module.css'

const accountSecurity = new AccountSecurityHttpRepository()

/** Password, sessions and account deactivation. */
export function SecuritySection() {
  const { message } = App.useApp()
  const navigate = useNavigate()
  const changePassword = useAuthStore((s) => s.changePassword)
  const [isSavingPassword, setIsSavingPassword] = useState(false)
  const [sessions, setSessions] = useState<AccountSession[] | null>(null)
  const [sessionsError, setSessionsError] = useState<string | null>(null)
  const [isSavingSessions, setIsSavingSessions] = useState(false)
  const [isDeactivateOpen, setIsDeactivateOpen] = useState(false)
  const [deactivatePassword, setDeactivatePassword] = useState('')
  const [isDeactivating, setIsDeactivating] = useState(false)

  const loadSessions = useCallback(async () => {
    try {
      setSessions(await accountSecurity.listSessions())
      setSessionsError(null)
    } catch (error) {
      setSessionsError(extractErrorMessage(error, 'No se pudieron cargar las sesiones'))
    }
  }, [])

  useEffect(() => {
    let active = true
    accountSecurity
      .listSessions()
      .then((list) => { if (active) { setSessions(list); setSessionsError(null) } })
      .catch((error) => { if (active) setSessionsError(extractErrorMessage(error, "No se pudieron cargar las sesiones")) })
    return () => { active = false }
  }, [])

  const handlePasswordSubmit = async (payload: ChangePasswordPayload) => {
    setIsSavingPassword(true)
    try {
      await changePassword(payload)
      message.success('Contraseña cambiada')
    } catch (error) {
      message.error(extractErrorMessage(error, 'No se pudo cambiar la contraseña'))
      throw error
    } finally {
      setIsSavingPassword(false)
    }
  }

  const handleRevokeSession = async (id: string) => {
    setIsSavingSessions(true)
    try {
      await accountSecurity.revokeSession(id)
      message.success('Sesión cerrada')
      await loadSessions()
    } catch (error) {
      message.error(extractErrorMessage(error, 'No se pudo cerrar la sesión'))
    } finally {
      setIsSavingSessions(false)
    }
  }

  const handleRevokeOthers = async () => {
    setIsSavingSessions(true)
    try {
      await accountSecurity.revokeOtherSessions()
      message.success('Las demás sesiones se cerraron')
      await loadSessions()
    } catch (error) {
      message.error(extractErrorMessage(error, 'No se pudieron cerrar las sesiones'))
    } finally {
      setIsSavingSessions(false)
    }
  }

  const handleDeactivate = async () => {
    setIsDeactivating(true)
    try {
      await accountSecurity.deactivateAccount(deactivatePassword)
      useAuthStore.getState().logout()
      message.success('Tu cuenta fue desactivada')
      navigate('/login', { replace: true })
    } catch (error) {
      message.error(extractErrorMessage(error, 'No se pudo desactivar la cuenta'))
    } finally {
      setIsDeactivating(false)
    }
  }

  return (
    <section id="perfil-seguridad" aria-labelledby="perfil-seguridad-title" className={styles.section}>
      <Typography.Title level={4} id="perfil-seguridad-title" className={styles.sectionTitle}>Seguridad</Typography.Title>
      <Card title="Contraseña" className={styles.card}>
        <PasswordForm isSubmitting={isSavingPassword} onSubmit={handlePasswordSubmit} />
      </Card>
      <Card className={styles.card}>
        <div className={styles.rowBetween}>
          <div>
            <Typography.Text strong>Verificación en dos pasos</Typography.Text>
            <Typography.Paragraph type="secondary" className={styles.rowHint}>
              Añade una capa extra de seguridad al iniciar sesión.
            </Typography.Paragraph>
          </div>
          <Space>
            <Tag color="default">Próximamente</Tag>
            <Switch disabled aria-label="Verificación en dos pasos" />
          </Space>
        </div>
      </Card>
      <Card title="Sesiones y accesos" className={styles.card}>
        {sessionsError ? (
          <Alert type="error" showIcon message={sessionsError} />
        ) : sessions === null ? (
          <Skeleton active paragraph={{ rows: 2 }} />
        ) : (
          <div className={styles.sessionList}>
            {sessions.map((session) => (
              <div key={session.id} className={styles.sessionItem}>
                <div className={styles.rowBetween}>
                  <div>
                    <Typography.Text strong>{describeUserAgent(session.userAgent)}</Typography.Text>
                    {session.current && (
                      <Tag color="success" className={styles.currentTag}><CheckCircleFilled /> Esta sesión</Tag>
                    )}
                    <Typography.Paragraph type="secondary" className={styles.rowHint}>
                      Iniciada {dayjs(session.createdAt).format('DD/MM/YYYY HH:mm')} · Última actividad {dayjs(session.lastSeenAt).format('DD/MM/YYYY HH:mm')}
                    </Typography.Paragraph>
                  </div>
                  {!session.current && (
                    <Button onClick={() => void handleRevokeSession(session.id)} loading={isSavingSessions}>Cerrar</Button>
                  )}
                </div>
              </div>
            ))}
          </div>
        )}
        {sessions?.some((session) => !session.current) && (
          <div className={styles.rowBetween}>
            <Typography.Paragraph type="secondary" className={styles.rowHint}>
              Cierra las sesiones que no reconozcas.
            </Typography.Paragraph>
            <Popconfirm
              title="¿Cerrar las demás sesiones?"
              description="Tendrás que volver a iniciar sesión en esos dispositivos."
              okText="Sí, cerrar"
              cancelText="No"
              onConfirm={() => void handleRevokeOthers()}
            >
              <Button loading={isSavingSessions}>Cerrar otras sesiones</Button>
            </Popconfirm>
          </div>
        )}
      </Card>
      <Card className={styles.dangerCard} title="Zona de peligro">
        <div className={styles.rowBetween}>
          <div>
            <Typography.Text strong>Eliminar mi cuenta</Typography.Text>
            <Typography.Paragraph type="secondary" className={styles.rowHint}>
              Tu cuenta quedará desactivada y no podrás volver a entrar. Tus incidencias se conservan con tu nombre de solicitante.
            </Typography.Paragraph>
          </div>
          <Button danger icon={<DeleteOutlined />} onClick={() => setIsDeactivateOpen(true)}>Eliminar cuenta</Button>
        </div>
      </Card>
      <Modal
        title="Eliminar mi cuenta"
        open={isDeactivateOpen}
        onCancel={() => { setIsDeactivateOpen(false); setDeactivatePassword('') }}
        onOk={() => void handleDeactivate()}
        okText="Eliminar cuenta"
        cancelText="Cancelar"
        okButtonProps={{ danger: true, disabled: deactivatePassword.length === 0, loading: isDeactivating }}
        destroyOnHidden
      >
        <Typography.Paragraph>
          Se cerrarán todas tus sesiones. Para confirmar, escribe tu contraseña actual.
        </Typography.Paragraph>
        <Input.Password
          aria-label="Contraseña actual"
          placeholder="Contraseña actual"
          value={deactivatePassword}
          onChange={(event) => setDeactivatePassword(event.target.value)}
        />
      </Modal>
    </section>
  )
}
