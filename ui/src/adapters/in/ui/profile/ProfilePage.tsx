import { BellOutlined, BgColorsOutlined, KeyOutlined, LockOutlined, UserOutlined } from '@ant-design/icons'
import { Card, Grid, Menu, Segmented, Space, Tag, Typography } from 'antd'
import type { MenuProps } from 'antd'
import dayjs from 'dayjs'
import { useCallback, type ReactNode } from 'react'
import { useNavigate, useParams } from 'react-router-dom'

import { useAuthStore } from '@/adapters/in/state/auth.store'
import type { User } from '@/core/auth/domain/user'
import { PageContainer } from '@/shared/ui/PageContainer'
import { UserAvatar } from '../people/UserAvatar'
import { AppearanceSection } from './AppearanceSection'
import { GeneralSection } from './GeneralSection'
import { NotificationsSection } from './NotificationsSection'
import { SecuritySection } from './SecuritySection'
import { TokensSection } from './TokensSection'
import styles from './ProfilePage.module.css'

type SectionKey = 'general' | 'security' | 'notifications' | 'tokens' | 'appearance'

const SECTIONS: { key: SectionKey; label: string; icon: ReactNode }[] = [
  { key: 'general', label: 'General', icon: <UserOutlined /> },
  { key: 'security', label: 'Seguridad', icon: <LockOutlined /> },
  { key: 'notifications', label: 'Notificaciones', icon: <BellOutlined /> },
  { key: 'tokens', label: 'Tokens', icon: <KeyOutlined /> },
  { key: 'appearance', label: 'Apariencia', icon: <BgColorsOutlined /> },
]

const roleLabels: Record<User['role'], string> = {
  admin: 'Administrador',
  user: 'Usuario',
}

function sectionOf(value: string | undefined): SectionKey {
  return SECTIONS.some((section) => section.key === value) ? (value as SectionKey) : 'general'
}

/** Profile page: left section navigation under /profile/:section (general when no section). */
export function ProfilePage() {
  const screens = Grid.useBreakpoint()
  const isDesktop = screens.lg !== false
  const navigate = useNavigate()
  const { section } = useParams<{ section?: string }>()
  const active = sectionOf(section)
  const user = useAuthStore((s) => s.user)
  const setUser = useCallback((next: User) => useAuthStore.setState({ user: next }), [])

  if (!user) {
    return null
  }

  // Leaving a section with unsaved changes is guarded by the form (useUnsavedChangesGuard).
  const goTo = (key: SectionKey) => {
    if (key === active) return
    navigate(`/profile/${key}`)
  }

  const navItems: MenuProps['items'] = SECTIONS.map((item) => ({ key: item.key, icon: item.icon, label: item.label }))
  const memberSince = user.createdAt ? dayjs(user.createdAt).format('DD/MM/YYYY') : undefined
  const shownName = user.displayName || user.name

  return (
    <PageContainer variant="reading" pageName="profile" ready>
      <div className={styles.page}>
      <Card className={styles.summary} aria-label="Resumen de la cuenta">
        <div className={styles.summaryMain}>
          <UserAvatar id={user.id} name={shownName} avatarKey={user.avatarKey} size={64} />
          <div className={styles.identity}>
            <Typography.Title level={4} className={styles.name}>{shownName}</Typography.Title>
            <Typography.Text type="secondary">{user.email}</Typography.Text>
          </div>
        </div>
        <Space wrap size={[8, 8]}>
          <Tag color="blue">{roleLabels[user.role]}</Tag>
          {memberSince && <Tag>Miembro desde {memberSince}</Tag>}
        </Space>
      </Card>

      <div className={styles.layout}>
        <nav className={styles.sideNav} aria-label="Secciones de la cuenta">
          {isDesktop ? (
            <Menu
              mode="inline"
              selectedKeys={[active]}
              items={navItems}
              onClick={({ key }) => goTo(key as SectionKey)}
              className={styles.menu}
            />
          ) : (
            <div className={styles.tabsScroll} data-scroll-x="">
              <Segmented
                aria-label="Secciones de la cuenta"
                value={active}
                onChange={(value) => goTo(value as SectionKey)}
                options={SECTIONS.map((item) => ({
                  value: item.key,
                  label: (
                    <span className={styles.tabLabel}>
                      {item.icon}
                      {item.label}
                    </span>
                  ),
                }))}
              />
            </div>
          )}
        </nav>

        <div className={styles.sections}>
          {active === 'general' && (
            <GeneralSection user={user} onUserChange={setUser} />
          )}
          {active === 'security' && <SecuritySection />}
          {active === 'notifications' && <NotificationsSection />}
          {active === 'tokens' && <TokensSection />}
          {active === 'appearance' && <AppearanceSection />}
        </div>
      </div>
      </div>
    </PageContainer>
  )
}
