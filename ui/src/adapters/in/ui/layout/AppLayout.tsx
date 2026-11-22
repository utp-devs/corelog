import {
  DashboardOutlined,
  LogoutOutlined,
  MenuFoldOutlined,
  MenuUnfoldOutlined,
  MoonOutlined,
  SettingOutlined,
  SunOutlined,
  TeamOutlined,
  UnorderedListOutlined,
  UserOutlined,
  ProjectOutlined,
} from '@ant-design/icons'
import { Avatar, Dropdown, Layout, Menu, Space, Typography, Button, Grid } from 'antd'
import type { MenuProps } from 'antd'
import { Link, Outlet, useLocation, useNavigate } from 'react-router-dom'
import { useState } from 'react'

import { useAuthStore } from '@/adapters/in/state/auth.store'
import { navigationItems, type NavigationItem } from '@/app/navigation'
import { useThemeMode } from '@/app/theme-context'
import styles from './AppLayout.module.css'
import { GlobalSearch } from './GlobalSearch'
import { NotificationsBell } from './NotificationsBell'

const { Header, Sider, Content } = Layout

function resolveSelectedKey(pathname: string): string {
  if (pathname.startsWith('/dashboard')) return '/dashboard'
  if (pathname.startsWith('/projects')) return '/projects'
  if (pathname.startsWith('/issues/new')) return '/issues/new'
  if (pathname.startsWith('/issues')) return '/issues'
  if (pathname.startsWith('/teams')) return '/teams'
  if (pathname.startsWith('/profile')) return '/profile'
  return '/issues'
}

const breadcrumbLabels: Record<string, string> = {
  '/dashboard': 'Mi resumen',
  '/projects': 'Proyectos',
  '/issues': 'Incidencias',
  '/teams': 'Equipos',
  '/profile': 'Mi perfil',
}

const iconMap = {
  dashboard: <DashboardOutlined />,
  tickets: <UnorderedListOutlined />,
  profile: <SettingOutlined />,
  teams: <TeamOutlined />,
  projects: <ProjectOutlined />,
}

export function AppLayout() {
  const location = useLocation()
  const navigate = useNavigate()
  const screens = Grid.useBreakpoint()
  const [collapsed, setCollapsed] = useState(false)
  const { themeMode, toggleTheme } = useThemeMode()
  const user = useAuthStore((s) => s.user)
  const logout = useAuthStore((s) => s.logout)

  const selectedKey = resolveSelectedKey(location.pathname)
  const role = user?.role === 'admin' ? 'admin' : user?.role === 'agent' ? 'agent' : 'requester'
  const groups = ['Trabajo', 'Administración', 'Cuenta'] as const
  const menuItems: MenuProps['items'] = groups
    .map((group) => ({
      type: 'group' as const,
      key: group,
      label: group,
      children: navigationItems
        .filter((item: NavigationItem) => item.group === group && item.roles.includes(role))
        .map((item) => ({
          key: item.key,
          icon: iconMap[item.icon],
          label: <Link to={item.key}>{item.label}</Link>,
        })),
    }))
    .filter((group) => group.children.length > 0)

  const userMenuItems: MenuProps['items'] = [
    { key: 'profile', icon: <UserOutlined />, label: 'Mi perfil', onClick: () => navigate('/profile') },
    { key: 'logout', icon: <LogoutOutlined />, danger: true, label: 'Cerrar sesion', onClick: () => { logout(); navigate('/login') } },
  ]

  const isMobile = !screens.lg

  return (
    <Layout className={styles.layout}>
      {!isMobile && (
        <Sider className={styles.sider} collapsible collapsed={collapsed} onCollapse={setCollapsed} width={256}>
          <Link
            to="/issues"
            className={collapsed ? `${styles.brand} ${styles.brandCollapsed}` : styles.brand}
            aria-label="CoreLog"
          >
            {collapsed ? (
              <img
                src="/brand/corelog-icon-64.png"
                srcSet="/brand/corelog-icon-64.png 1x, /brand/corelog-icon-128.png 2x"
                alt=""
                width={40}
                height={40}
                className={styles.brandIcon}
              />
            ) : (
              <img
                src="/brand/corelog-banner-480.png"
                srcSet="/brand/corelog-banner-480.png 480w, /brand/corelog-banner-960.png 960w"
                sizes="152px"
                alt="CoreLog"
                width={152}
                className={styles.brandBanner}
              />
            )}
          </Link>
          <Menu mode="inline" selectedKeys={[selectedKey]} items={menuItems} />
          <Button
            className={styles.collapseButton}
            type="text"
            icon={collapsed ? <MenuUnfoldOutlined /> : <MenuFoldOutlined />}
            onClick={() => setCollapsed((value) => !value)}
            aria-label={collapsed ? 'Expandir navegacion' : 'Contraer navegacion'}
          />
        </Sider>
      )}
      <Layout>
        <Header className={styles.header}>
          <div className={styles.headerStart}>
            {isMobile && (
              <Button type="text" icon={<MenuUnfoldOutlined />} aria-label="Abrir navegacion" />
            )}
            <Typography.Text className={styles.breadcrumb}>
              CoreLog / {breadcrumbLabels[selectedKey] ?? 'Proyectos'}
            </Typography.Text>
          </div>
          <Space size="small">
            <GlobalSearch />
            <NotificationsBell />
            <Button
              type="text"
              icon={themeMode === 'dark' ? <SunOutlined /> : <MoonOutlined />}
              onClick={toggleTheme}
              aria-label={themeMode === 'dark' ? 'Usar tema claro' : 'Usar tema oscuro'}
            />
            <Dropdown menu={{ items: userMenuItems }} trigger={['click']} placement="bottomRight">
              <Button type="text" className={styles.userButton}>
                <Avatar size="small">{user?.name?.charAt(0).toUpperCase()}</Avatar>
                {!isMobile && <span>{user?.name}</span>}
              </Button>
            </Dropdown>
          </Space>
        </Header>
        <Content className={styles.content}>
          <Outlet />
        </Content>
      </Layout>
    </Layout>
  )
}
