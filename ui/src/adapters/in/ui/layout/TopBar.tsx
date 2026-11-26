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

export const MOBILE_QUERY = '(max-width: 767.98px)'

export type TopBarProps = {
  org?: OrgInfo | null
  /** Real organization switcher (P2). Replaces the plain organization name when given. */
  orgSwitcherSlot?: ReactNode
  /** Opens the navigation drawer. The button is shown only when provided (mobile). */
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

/** Global header: logo, primary navigation with dropdowns, Create, search, bell, notes, help and user menu. */
export function TopBar({ org = null, orgSwitcherSlot, onOpenMenu, onOpenShortcuts }: TopBarProps) {
  const user = useAuthStore((s) => s.user)
  const isMobile = useMediaQuery(MOBILE_QUERY)
  const [createOpen, setCreateOpen] = useState(false)
  const nav = buildTopNav(user, org)

  const orgSlot = orgSwitcherSlot ?? (nav.orgName ? <span className={styles.orgName}>{nav.orgName}</span> : null)

  return (
    <header className={styles.bar}>
      {onOpenMenu && (
        <Button type="text" icon={<MenuOutlined />} onClick={onOpenMenu} aria-label="Abrir menú de navegación" />
      )}

      <Link to="/issues" className={styles.brand} aria-label="CoreLog, ir a incidencias">
        <img
          src="/brand/corelog-banner-480.png"
          srcSet="/brand/corelog-banner-480.png 480w, /brand/corelog-banner-960.png 960w"
          sizes="128px"
          alt=""
          width={128}
          className={styles.brandBanner}
        />
      </Link>

      {!isMobile && (
        <nav aria-label="Principal" className={styles.nav}>
          {nav.menus.map((menu) => (
            <NavMenuDropdown key={menu.key} menu={menu} />
          ))}
        </nav>
      )}

      {nav.canCreate && (
        <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreateOpen(true)} className={styles.create}>
          Crear
        </Button>
      )}

      <div className={styles.spacer} />

      {orgSlot && <div className={styles.org}>{orgSlot}</div>}

      <GlobalSearch />
      <NotificationsBell />
      <NotesButton />
      <HelpMenu onOpenShortcuts={onOpenShortcuts} />
      <UserMenu />

      <CreateIssueModal open={createOpen} onClose={() => setCreateOpen(false)} />
    </header>
  )
}
