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
import { App, Avatar, Button, Card, Descriptions, Grid, Menu, Segmented, Select, Space, Switch, Tag, Typography } from 'antd'
import type { MenuProps } from 'antd'
import dayjs from 'dayjs'
import { useState } from 'react'

import type { ChangePasswordPayload, UpdateProfilePayload } from '@/core/auth/application/ports/auth-repository.port'
import { useAuthStore } from '@/adapters/in/state/auth.store'
import { extractErrorMessage } from '@/adapters/out/http/api-client'
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

  if (!user) {
    return null
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
                  <Space>
                    <PendingTag />
                    <Switch disabled defaultChecked aria-label="Avisos en la aplicación" />
                  </Space>
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
              <div className={styles.rowBetween}>
                <div>
                  <Typography.Text strong>Este navegador</Typography.Text>
                  <Typography.Paragraph type="secondary" className={styles.rowHint}>
                    <CheckCircleFilled className={styles.okIcon} /> Sesión actual
                  </Typography.Paragraph>
                </div>
                <Button disabled>Cerrar otras sesiones</Button>
              </div>
              <Typography.Paragraph type="secondary" className={styles.hint}>
                El historial completo de sesiones estará disponible próximamente.
              </Typography.Paragraph>
            </Card>
          </section>

          <section id="perfil-zona" className={styles.section}>
            <Typography.Title level={4} className={styles.sectionTitle}>Zona de peligro</Typography.Title>
            <Card className={styles.dangerCard}>
              <div className={styles.rowBetween}>
                <div>
                  <Typography.Text strong>Eliminar mi cuenta</Typography.Text>
                  <Typography.Paragraph type="secondary" className={styles.rowHint}>
                    Esta acción no se puede deshacer. Tus incidencias se conservan con tu nombre de solicitante.
                  </Typography.Paragraph>
                </div>
                <Space>
                  <PendingTag />
                  <Button danger icon={<DeleteOutlined />} disabled>Eliminar cuenta</Button>
                </Space>
              </div>
            </Card>
          </section>
        </div>
      </div>
    </div>
  )
}
