import {
  DashboardOutlined,
  LogoutOutlined,
  MenuFoldOutlined,
  MenuUnfoldOutlined,
  MoonOutlined,
  ProjectOutlined,
  SettingOutlined,
  SunOutlined,
  TeamOutlined,
  UnorderedListOutlined,
  UserOutlined,
} from '@ant-design/icons'
import { Avatar, Button, Drawer, Dropdown, Grid, Layout, Menu, Space, Tag, Typography } from 'antd'
import type { MenuProps } from 'antd'
import { useState } from 'react'
import { Link, Outlet, useLocation, useNavigate } from 'react-router-dom'

import { useAuthStore } from '@/adapters/in/state/auth.store'
import { navigationItems, type NavigationGroup, type NavigationItem, type NavigationRole } from '@/app/navigation'
import { useThemeMode } from '@/app/theme-context'
import { useGlobalShortcuts } from '@/shared/hooks/useGlobalShortcuts'
import styles from './AppLayout.module.css'
import { GlobalSearch } from './GlobalSearch'
import { NotificationsBell } from './NotificationsBell'

const { Header, Sider, Content } = Layout

const groups: NavigationGroup[] = ['Trabajo', 'Administración', 'Cuenta']

const roleLabels: Record<NavigationRole, string> = {
  admin: 'Administrador',
  agent: 'Agente',
  requester: 'Solicitante',
}

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
  // En escritorio el menu es fijo; por debajo de lg se usa un cajon deslizable.
  const isDesktop = screens.lg !== false
  const [collapsed, setCollapsed] = useState(false)
  const [drawerOpen, setDrawerOpen] = useState(false)
  const { themeMode, toggleTheme } = useThemeMode()
  const user = useAuthStore((s) => s.user)
  const logout = useAuthStore((s) => s.logout)

  useGlobalShortcuts()
  const selectedKey = resolveSelectedKey(location.pathname)
  const role: NavigationRole = user?.role === 'admin' ? 'admin' : user?.role === 'agent' ? 'agent' : 'requester'

  const menuItems: MenuProps['items'] = groups
    .map((group) => ({
      type: 'group' as const,
      key: group,
      label: collapsed && isDesktop ? <span className={styles.groupDivider} aria-label={group} /> : group,
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

  const navigation = (
    <Menu
      mode="inline"
      selectedKeys={[selectedKey]}
      items={menuItems}
      onClick={() => setDrawerOpen(false)}
      className={styles.menu}
    />
  )

  const brand = (
    <Link to="/issues" className={collapsed && isDesktop ? `${styles.brand} ${styles.brandCollapsed}` : styles.brand} aria-label="CoreLog">
      {collapsed && isDesktop ? (
        <img
          src="/brand/corelog-icon-64.png"
          srcSet="/brand/corelog-icon-64.png 1x, /brand/corelog-icon-128.png 2x"
          alt=""
          width={36}
          height={36}
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
  )

  return (
    <Layout className={styles.shell}>
      {isDesktop && (
        <Sider
          className={styles.sider}
          width={248}
          collapsedWidth={72}
          collapsed={collapsed}
          trigger={null}
          aria-label="Navegacion principal"
        >
          <div className={styles.siderInner}>
            {brand}
            <nav className={styles.nav}>{navigation}</nav>
            <div className={styles.siderFooter}>
              <Button
                type="text"
                block={!collapsed}
                icon={collapsed ? <MenuUnfoldOutlined /> : <MenuFoldOutlined />}
                onClick={() => setCollapsed((value) => !value)}
                aria-label={collapsed ? 'Expandir navegacion' : 'Contraer navegacion'}
              >
                {collapsed ? null : 'Contraer menu'}
              </Button>
            </div>
          </div>
        </Sider>
      )}

      {!isDesktop && (
        <Drawer
          title={brand}
          placement="left"
          width={272}
          open={drawerOpen}
          onClose={() => setDrawerOpen(false)}
          aria-label="Navegacion principal"
        >
          {navigation}
        </Drawer>
      )}

      <Layout className={styles.main}>
        <Header className={styles.header}>
          <div className={styles.headerStart}>
            {!isDesktop && (
              <Button
                type="text"
                icon={<MenuUnfoldOutlined />}
                onClick={() => setDrawerOpen(true)}
                aria-label="Abrir navegacion"
              />
            )}
            <Typography.Text className={styles.breadcrumb}>
              CoreLog / <span className={styles.breadcrumbCurrent}>{breadcrumbLabels[selectedKey] ?? 'Proyectos'}</span>
            </Typography.Text>
          </div>
          <Space size={8} className={styles.headerEnd}>
            <GlobalSearch />
            <NotificationsBell />
            <Button
              type="text"
              icon={themeMode === 'dark' ? <SunOutlined /> : <MoonOutlined />}
              onClick={toggleTheme}
              aria-label={themeMode === 'dark' ? 'Usar tema claro' : 'Usar tema oscuro'}
            />
            <Dropdown menu={{ items: userMenuItems }} trigger={['click']} placement="bottomRight">
              <Button type="text" className={styles.userButton} aria-label="Menu de usuario">
                <Avatar size="small">{user?.name?.charAt(0).toUpperCase()}</Avatar>
                {isDesktop && (
                  <span className={styles.userText}>
                    <span className={styles.userName}>{user?.name}</span>
                    {user && <Tag className={styles.roleTag}>{roleLabels[role]}</Tag>}
                  </span>
                )}
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
