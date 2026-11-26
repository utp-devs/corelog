import { DownOutlined, MenuOutlined, PlusOutlined } from '@ant-design/icons'
import { Button, Dropdown } from 'antd'
import type { MenuProps } from 'antd'
import { useState, type ReactNode } from 'react'
import { Link, useNavigate } from 'react-router-dom'

import { useAuthStore } from '@/adapters/in/state/auth.store'
import { buildTopNav, VIEW_ALL_LABEL, type OrgInfo, type TopNavMenu } from '@/app/navigation'
import { useMediaQuery } from '@/shared/hooks/useMediaQuery'

import { CreateIssueModal } from './CreateIssueModal'
import { GlobalSearch } from './GlobalSearch'
import { HelpMenu } from './HelpMenu'
import { NotesButton } from './NotesButton'
import { NotificationsBell } from './NotificationsBell'
import styles from './TopBar.module.css'
import { UserMenu } from './UserMenu'

/** Phones: icon-only Create, logo icon, organization switcher moves into the drawer. */
export const MOBILE_QUERY = '(max-width: 767.98px)'
/** Below 1024 px the primary menus collapse into the navigation drawer. */
export const COLLAPSED_NAV_QUERY = '(max-width: 1023.98px)'
/** Below 1280 px the search field becomes an icon that opens the command palette. */
export const ICON_SEARCH_QUERY = '(max-width: 1279.98px)'

export type TopBarProps = {
  org?: OrgInfo | null
  /** Real organization switcher (P2). Replaces the plain organization name when given. */
  orgSwitcherSlot?: ReactNode
  /** Opens the navigation drawer. The button is shown only when provided (below 1024 px). */
  onOpenMenu?: () => void
  onOpenShortcuts: () => void
}

function NavMenuDropdown({ menu }: { menu: TopNavMenu }) {
  const navigate = useNavigate()
  const items: MenuProps['items'] = [
    {
      type: 'group',
      key: 'recent',
      label: 'Recientes',
      children: menu.recent.length > 0
        ? menu.recent.map((link) => ({ key: link.key, label: link.label, onClick: () => navigate(link.path) }))
        : [{ key: 'none', label: 'Sin elementos recientes', disabled: true }],
    },
    { type: 'divider', key: 'divider' },
    { key: 'all', label: VIEW_ALL_LABEL, onClick: () => navigate(menu.path) },
    ...(menu.extras ?? []).map((link) => ({ key: link.key, label: link.label, onClick: () => navigate(link.path) })),
  ]

  return (
    <Dropdown menu={{ items }} trigger={['click']}>
      <Button type="text" className={styles.navButton} aria-haspopup="menu">
        {menu.label}
        <DownOutlined className={styles.caret} aria-hidden="true" />
      </Button>
    </Dropdown>
  )
}

/** Global header: logo, primary navigation with dropdowns, Create, search, bell, help and user menu. */
export function TopBar({ org = null, orgSwitcherSlot, onOpenMenu, onOpenShortcuts }: TopBarProps) {
  const user = useAuthStore((s) => s.user)
  const isMobile = useMediaQuery(MOBILE_QUERY)
  const collapsedNav = useMediaQuery(COLLAPSED_NAV_QUERY)
  const iconSearch = useMediaQuery(ICON_SEARCH_QUERY)
  const [createOpen, setCreateOpen] = useState(false)
  const nav = buildTopNav(user, org)

  const orgSlot = orgSwitcherSlot ?? (nav.orgName ? <span className={styles.orgName}>{nav.orgName}</span> : null)

  return (
    <header className={styles.bar}>
      {onOpenMenu && (
        <Button type="text" icon={<MenuOutlined />} onClick={onOpenMenu} aria-label="Abrir menú de navegación">
          {isMobile ? null : 'Menú'}
        </Button>
      )}

      <Link to="/issues" className={styles.brand} aria-label="CoreLog, ir a incidencias">
        {isMobile ? (
          <img
            src="/brand/corelog-icon-64.png"
            srcSet="/brand/corelog-icon-64.png 64w, /brand/corelog-icon-128.png 128w"
            sizes="32px"
            alt=""
            width={32}
            height={32}
            className={styles.brandIcon}
          />
        ) : (
          <img
            src="/brand/corelog-banner-480.png"
            srcSet="/brand/corelog-banner-480.png 480w, /brand/corelog-banner-960.png 960w"
            sizes="128px"
            alt=""
            width={128}
            className={styles.brandBanner}
          />
        )}
      </Link>

      {!collapsedNav && (
        <nav aria-label="Principal" className={styles.nav}>
          {nav.menus.map((menu) => (
            <NavMenuDropdown key={menu.key} menu={menu} />
          ))}
        </nav>
      )}

      {nav.canCreate && (
        isMobile ? (
          <Button
            type="primary"
            icon={<PlusOutlined />}
            onClick={() => setCreateOpen(true)}
            aria-label="Crear"
            className={styles.create}
          />
        ) : (
          <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreateOpen(true)} className={styles.create}>
            Crear
          </Button>
        )
      )}

      <div className={styles.spacer} />

      {orgSlot && !isMobile && <div className={styles.org}>{orgSlot}</div>}

      <GlobalSearch compact={iconSearch} />
      <NotificationsBell />
      {!isMobile && <NotesButton />}
      <HelpMenu onOpenShortcuts={onOpenShortcuts} />
      <UserMenu />

      <CreateIssueModal open={createOpen} onClose={() => setCreateOpen(false)} />
    </header>
  )
}
