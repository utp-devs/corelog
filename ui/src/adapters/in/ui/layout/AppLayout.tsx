import { Drawer } from 'antd'
import { useEffect, useState, type MouseEvent, type ReactNode } from 'react'
import { Link, NavLink, Outlet, useLocation } from 'react-router-dom'

import { useAuthStore } from '@/adapters/in/state/auth.store'
import { useProjectsStore } from '@/adapters/in/state/projects.store'
import {
  buildSidebar,
  buildTopNav,
  isSidebarItemActive,
  permissionsFor,
  resolveSidebarContext,
  type OrgInfo,
} from '@/app/navigation'
import { useGlobalShortcuts } from '@/shared/hooks/useGlobalShortcuts'
import { useMediaQuery } from '@/shared/hooks/useMediaQuery'
import { ShortcutsDialog } from '@/shared/ui/ShortcutsDialog'

import { CommandPaletteHost } from './CommandPaletteHost'
import { ContextSidebar, SidebarItemIcon } from './ContextSidebar'
import styles from './AppLayout.module.css'
import { MOBILE_QUERY, TopBar } from './TopBar'

export type AppLayoutProps = {
  org?: OrgInfo | null
  /** Organization switcher of P2; until then the top bar shows the organization name. */
  orgSwitcherSlot?: ReactNode
}

const CONTEXT_TITLES = {
  project: 'Proyecto',
  'org-admin': 'Organización',
  profile: 'Mi cuenta',
} as const

/** Application shell: skip link, top bar, context sidebar (or mobile drawer) and the main landmark. */
export function AppLayout({ org = null, orgSwitcherSlot }: AppLayoutProps) {
  const location = useLocation()
  const user = useAuthStore((s) => s.user)
  const isMobile = useMediaQuery(MOBILE_QUERY)
  const [helpOpen, setHelpOpen] = useState(false)
  const [drawerOpen, setDrawerOpen] = useState(false)

  const context = resolveSidebarContext(location.pathname)
  const projectId = context.kind === 'project' ? context.projectId : null
  const project = useProjectsStore((s) => (projectId ? s.projects.find((p) => p.id === projectId) : undefined))
  const hasProject = project !== undefined
  const items = buildSidebar(context, permissionsFor(user, org?.role))

  // The project header needs the projects list; load it when the URL points at a project we do not have yet.
  useEffect(() => {
    if (projectId && !hasProject) {
      void useProjectsStore.getState().fetchProjects()
    }
  }, [projectId, hasProject])
  const topMenus = buildTopNav(user, org).menus
  const title = context.kind === 'global' ? '' : CONTEXT_TITLES[context.kind === 'project' ? 'project' : context.kind]

  useGlobalShortcuts({ onHelp: () => setHelpOpen((open) => !open), projectId })

  // The skip link moves focus to the main landmark instead of only changing the hash.
  const skipToContent = (event: MouseEvent<HTMLAnchorElement>) => {
    event.preventDefault()
    document.getElementById('main-content')?.focus()
  }

  return (
    <div className={styles.shell}>
      <a href="#main-content" className={styles.skip} onClick={skipToContent}>
        Saltar al contenido
      </a>

      <TopBar
        org={org}
        orgSwitcherSlot={orgSwitcherSlot}
        onOpenMenu={isMobile ? () => setDrawerOpen(true) : undefined}
        onOpenShortcuts={() => setHelpOpen(true)}
      />

      <div className={styles.body}>
        {!isMobile && context.kind !== 'global' && (
          <ContextSidebar
            title={title}
            items={items}
            project={project ? { id: project.id, name: project.name, key: project.key } : undefined}
          />
        )}

        <main id="main-content" tabIndex={-1} className={styles.main}>
          <Outlet />
        </main>
      </div>

      {isMobile && (
        <Drawer
          open={drawerOpen}
          onClose={() => setDrawerOpen(false)}
          placement="left"
          size={272}
          title="Navegación"
        >
          <nav aria-label="Navegación móvil" className={styles.drawerNav}>
            <ul className={styles.drawerList}>
              {topMenus.map((menu) => (
                <li key={menu.key}>
                  <Link to={menu.path} onClick={() => setDrawerOpen(false)} className={styles.drawerLink}>
                    {menu.label}
                  </Link>
                </li>
              ))}
            </ul>
            {items.length > 0 && (
              <>
                <p className={styles.drawerTitle}>{title}</p>
                <ul className={styles.drawerList}>
                  {items.map((item) => (
                    <li key={item.key}>
                      <NavLink
                        to={item.path}
                        onClick={() => setDrawerOpen(false)}
                        className={isSidebarItemActive(item, location.pathname) ? styles.drawerLinkActive : styles.drawerLink}
                      >
                        <SidebarItemIcon icon={item.icon} />
                        <span>{item.label}</span>
                      </NavLink>
                    </li>
                  ))}
                </ul>
              </>
            )}
          </nav>
        </Drawer>
      )}

      <CommandPaletteHost />
      <ShortcutsDialog open={helpOpen} onClose={() => setHelpOpen(false)} />
    </div>
  )
}
