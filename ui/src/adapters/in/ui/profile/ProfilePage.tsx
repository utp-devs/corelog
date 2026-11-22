import {
  CheckCircleFilled,
  DeleteOutlined,
  GlobalOutlined,
  LockOutlined,
  SafetyCertificateOutlined,
  BgColorsOutlined,
  BellOutlined,
  UserOutlined,
  WarningOutlined,
} from '@ant-design/icons'
import { Alert, App, Avatar, Button, Card, Descriptions, Grid, Input, Menu, Modal, Popconfirm, Segmented, Select, Skeleton, Space, Switch, Tag, Typography } from 'antd'
import type { MenuProps } from 'antd'
import dayjs from 'dayjs'
import { useCallback, useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'

import type { ChangePasswordPayload, UpdateProfilePayload } from '@/core/auth/application/ports/auth-repository.port'
import { useAuthStore } from '@/adapters/in/state/auth.store'
import { extractErrorMessage } from '@/adapters/out/http/api-client'
import { AccountSecurityHttpRepository } from '@/adapters/out/http/account-security-http-repository'
import { NotificationHttpRepository } from '@/adapters/out/http/notification-http-repository'
import { describeUserAgent, type AccountSession } from '@/core/auth/domain/session'
import type { ThemeMode } from '@/adapters/out/storage/theme-storage'
import { useThemeMode } from '@/app/theme-context'
import { PasswordForm } from './PasswordForm'
import { ProfileForm } from './ProfileForm'
import styles from './ProfilePage.module.css'

const themeOptions: { value: ThemeMode; label: string }[] = [
  { value: 'light', label: 'Claro' },
  { value: 'dark', label: 'Oscuro' },
]

const roleLabels: Record<string, string> = {
  admin: 'Administrador',
  agent: 'Agente',
  requester: 'Solicitante',
}

// Secciones de la pagina. El menu lateral hace scroll hasta cada una.
const sections = [
  { key: 'datos', label: 'Mi información', icon: <UserOutlined /> },
  { key: 'seguridad', label: 'Seguridad', icon: <LockOutlined /> },
  { key: 'preferencias', label: 'Preferencias', icon: <BgColorsOutlined /> },
  { key: 'sesiones', label: 'Sesiones y accesos', icon: <SafetyCertificateOutlined /> },
  { key: 'zona', label: 'Zona de peligro', icon: <WarningOutlined /> },
]

// Etiqueta para lo que aun no tiene conexion con el backend.
function PendingTag() {
  return <Tag color="default">Próximamente</Tag>
}

const accountSecurity = new AccountSecurityHttpRepository()
const notificationPrefs = new NotificationHttpRepository()

export function ProfilePage() {
  const { message } = App.useApp()
  const screens = Grid.useBreakpoint()
  const isDesktop = screens.lg !== false
  const user = useAuthStore((s) => s.user)
  const updateProfile = useAuthStore((s) => s.updateProfile)
  const changePassword = useAuthStore((s) => s.changePassword)
  const { themeMode, setThemeMode } = useThemeMode()

  const [isSavingProfile, setIsSavingProfile] = useState(false)
  const [isSavingPassword, setIsSavingPassword] = useState(false)
  const [activeSection, setActiveSection] = useState('datos')
  const navigate = useNavigate()
  // null = aun no cargado (o no disponible): el interruptor queda deshabilitado.
  const [inAppEnabled, setInAppEnabled] = useState<boolean | null>(null)
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
    notificationPrefs
      .getPreferences()
      .then((prefs) => { if (active) setInAppEnabled(prefs.inApp) })
      .catch(() => undefined)
    accountSecurity
      .listSessions()
      .then((list) => { if (active) { setSessions(list); setSessionsError(null) } })
      .catch((error) => { if (active) setSessionsError(extractErrorMessage(error, 'No se pudieron cargar las sesiones')) })
    return () => { active = false }
  }, [])

  if (!user) {
    return null
  }

  const handleInAppToggle = async (checked: boolean) => {
    const previous = inAppEnabled
    setInAppEnabled(checked)
    try {
      const saved = await notificationPrefs.updatePreferences(checked)
      setInAppEnabled(saved.inApp)
      message.success(saved.inApp ? 'Avisos en la aplicación activados' : 'Avisos en la aplicación desactivados')
    } catch (error) {
      setInAppEnabled(previous)
      message.error(extractErrorMessage(error, 'No se pudo guardar la preferencia'))
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

  const handleProfileSubmit = async (payload: UpdateProfilePayload) => {
    setIsSavingProfile(true)
    try {
      await updateProfile(payload)
      message.success('Perfil actualizado')
    } catch (error) {
      message.error(extractErrorMessage(error, 'No se pudo actualizar el perfil'))
    } finally {
      setIsSavingProfile(false)
    }
  }

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

  const goTo = (key: string) => {
    setActiveSection(key)
    document.getElementById(`perfil-${key}`)?.scrollIntoView({ behavior: 'smooth', block: 'start' })
  }

  const navItems: MenuProps['items'] = sections.map((section) => ({
    key: section.key,
    icon: section.icon,
    label: section.label,
    danger: section.key === 'zona',
  }))

  const memberSince = user.createdAt ? dayjs(user.createdAt).format('DD/MM/YYYY') : undefined

  return (
    <div className={styles.page}>
      <Card className={styles.summary} aria-label="Resumen de la cuenta">
        <div className={styles.summaryMain}>
          <Avatar size={64} className={styles.avatar}>{user.name.charAt(0).toUpperCase()}</Avatar>
          <div className={styles.identity}>
            <Typography.Title level={4} className={styles.name}>{user.name}</Typography.Title>
            <Typography.Text type="secondary">{user.email}</Typography.Text>
          </div>
        </div>
        <Space wrap size={[8, 8]}>
          <Tag color="blue">{roleLabels[user.role] ?? user.role}</Tag>
          {memberSince && <Tag>Miembro desde {memberSince}</Tag>}
        </Space>
      </Card>

      <div className={styles.layout}>
        <nav className={styles.sideNav} aria-label="Secciones de la cuenta">
          <Menu
            mode={isDesktop ? 'inline' : 'horizontal'}
            selectedKeys={[activeSection]}
            items={navItems}
            onClick={({ key }) => goTo(key)}
            className={styles.menu}
          />
        </nav>

        <div className={styles.sections}>
          <section id="perfil-datos" className={styles.section}>
            <Typography.Title level={4} className={styles.sectionTitle}>Datos personales</Typography.Title>
            <div className={styles.pairs}>
            <Card>
              <Typography.Paragraph type="secondary" className={styles.hint}>
                Este nombre y correo son los que ven tus compañeros en las incidencias.
              </Typography.Paragraph>
              <ProfileForm
                initialName={user.name}
                initialEmail={user.email}
                isSubmitting={isSavingProfile}
                onSubmit={handleProfileSubmit}
              />
            </Card>
            <Card title="Datos de la cuenta" className={styles.card}>
              <Descriptions
                column={1}
                size="small"
                items={[
                  { key: "role", label: "Rol", children: roleLabels[user.role] ?? user.role },
                  { key: "since", label: "Miembro desde", children: memberSince ?? "—" },
                  { key: "id", label: "Identificador", children: <Typography.Text copyable>{user.id}</Typography.Text> },
                ]}
              />
            </Card>
            </div>
          </section>

          <section id="perfil-seguridad" className={styles.section}>
            <Typography.Title level={4} className={styles.sectionTitle}>Seguridad</Typography.Title>
            <div className={styles.pairs}>
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
                  <PendingTag />
                  <Switch disabled aria-label="Verificación en dos pasos" />
                </Space>
              </div>
            </Card>
            </div>
          </section>

          <section id="perfil-preferencias" className={styles.section}>
            <Typography.Title level={4} className={styles.sectionTitle}>Preferencias</Typography.Title>
            <div className={styles.pairs}>
            <Card title="Apariencia" className={styles.card}>
              <Typography.Paragraph type="secondary" className={styles.hint}>
                El tema se guarda en este navegador y se aplica al abrir la aplicación.
              </Typography.Paragraph>
              <Segmented
                aria-label="Tema de la aplicación"
                options={themeOptions}
                value={themeMode}
                onChange={(value) => setThemeMode(value as ThemeMode)}
              />
            </Card>
            <Card className={styles.card}>
              <div className={styles.rowBetween}>
                <div>
                  <Typography.Text strong><GlobalOutlined /> Idioma</Typography.Text>
                  <Typography.Paragraph type="secondary" className={styles.rowHint}>
                    La interfaz está disponible en español.
                  </Typography.Paragraph>
                </div>
                <Space>
                  <PendingTag />
                  <Select
                    aria-label="Idioma"
                    disabled
                    value="es"
                    options={[{ value: 'es', label: 'Español' }]}
                    className={styles.languageSelect}
                  />
                </Space>
              </div>
            </Card>
            </div>
            <Card title={<><BellOutlined /> Notificaciones</>} className={styles.card}>
              <Space orientation="vertical" className={styles.fullWidth}>
                <div className={styles.rowBetween}>
                  <Typography.Text>Avisos en la aplicación</Typography.Text>
                  <Switch
                    checked={inAppEnabled ?? false}
                    disabled={inAppEnabled === null}
                    loading={inAppEnabled === null}
                    onChange={(checked) => void handleInAppToggle(checked)}
                    aria-label="Avisos en la aplicación"
                  />
                </div>
                <div className={styles.rowBetween}>
                  <Typography.Text>Correo electrónico</Typography.Text>
                  <Space>
                    <PendingTag />
                    <Switch disabled aria-label="Correo electrónico" />
                  </Space>
                </div>
              </Space>
            </Card>
          </section>

          <section id="perfil-sesiones" className={styles.section}>
            <Typography.Title level={4} className={styles.sectionTitle}>Sesiones y accesos</Typography.Title>
            <Card className={styles.card}>
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
          </section>

          <section id="perfil-zona" className={styles.section}>
            <Typography.Title level={4} className={styles.sectionTitle}>Zona de peligro</Typography.Title>
            <Card className={styles.dangerCard}>
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
        </div>
      </div>
    </div>
  )
}
