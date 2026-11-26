import {
  AppstoreOutlined,
  CalendarOutlined,
  MailOutlined,
  MenuFoldOutlined,
  MenuUnfoldOutlined,
  SafetyOutlined,
  SettingOutlined,
  TeamOutlined,
  ThunderboltOutlined,
  UnorderedListOutlined,
  UserOutlined,
} from '@ant-design/icons'
import { Button } from 'antd'
import { useState, type ReactElement } from 'react'
import { NavLink, useLocation } from 'react-router-dom'

import { isSidebarItemActive, type SidebarItem, type SidebarItemIcon as SidebarIconName } from '@/app/navigation'
import { useMediaQuery } from '@/shared/hooks/useMediaQuery'
import { ProjectAvatar } from '@/shared/ui/ProjectAvatar'

import { readSidebarCollapsed, writeSidebarCollapsed } from './preferences'
import styles from './ContextSidebar.module.css'

/** Below this width the sidebar starts collapsed. */
export const NARROW_QUERY = '(max-width: 1279.98px)'

export type SidebarProject = {
  id: string
  name: string
  key: string
}

export type ContextSidebarProps = {
  /** Context name, used as the accessible name of the aside and as header when there is no project. */
  title: string
  items: readonly SidebarItem[]
  project?: SidebarProject
}

const ICONS: Record<SidebarIconName, ReactElement> = {
  board: <AppstoreOutlined />,
  backlog: <UnorderedListOutlined />,
  sprints: <ThunderboltOutlined />,
  calendar: <CalendarOutlined />,
  permissions: <SafetyOutlined />,
  settings: <SettingOutlined />,
  members: <TeamOutlined />,
  invitations: <MailOutlined />,
  profile: <UserOutlined />,
}

/** Icon of a sidebar item; shared with the mobile drawer. */
export function SidebarItemIcon({ icon }: { icon: SidebarIconName }) {
  return <span className={styles.icon} aria-hidden="true">{ICONS[icon]}</span>
}

/** Sidebar of the current context: project header, item list and a collapse toggle. */
export function ContextSidebar({ title, items, project }: ContextSidebarProps) {
  const location = useLocation()
  const narrow = useMediaQuery(NARROW_QUERY)
  const [stored, setStored] = useState<boolean | null>(readSidebarCollapsed)
  const collapsed = stored ?? narrow

  const toggle = () => {
    const next = !collapsed
    writeSidebarCollapsed(next)
    setStored(next)
  }

  return (
    <aside
      aria-label={title}
      data-collapsed={collapsed ? 'true' : 'false'}
      className={collapsed ? styles.collapsed : styles.sidebar}
    >
      <div className={styles.header}>
        {project ? (
          <div className={styles.project}>
            <ProjectAvatar name={project.name} projectKey={project.key} size={32} />
            {!collapsed && (
              <span className={styles.projectText}>
                <span className={styles.projectName}>{project.name}</span>
                <span className={styles.projectKey}>{project.key}</span>
              </span>
            )}
          </div>
        ) : (
          !collapsed && <span className={styles.title}>{title}</span>
        )}
      </div>

      <nav aria-label={`Navegación: ${title}`} className={styles.nav}>
        <ul className={styles.list}>
          {items.map((item) => (
            <li key={item.key}>
              <NavLink
                to={item.path}
                aria-label={item.label}
                title={collapsed ? item.label : undefined}
                className={isSidebarItemActive(item, location.pathname) ? styles.linkActive : styles.link}
              >
                <SidebarItemIcon icon={item.icon} />
                {!collapsed && <span className={styles.label}>{item.label}</span>}
              </NavLink>
            </li>
          ))}
        </ul>
      </nav>

      <div className={styles.footer}>
        <Button
          type="text"
          block={!collapsed}
          onClick={toggle}
          aria-expanded={!collapsed}
          aria-label={collapsed ? 'Expandir menú lateral' : 'Contraer menú lateral'}
          icon={collapsed ? <MenuUnfoldOutlined /> : <MenuFoldOutlined />}
        >
          {collapsed ? null : 'Contraer menú'}
        </Button>
      </div>
    </aside>
  )
}
