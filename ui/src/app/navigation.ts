import type { Role, User } from '@/core/auth/domain/user'

/** Permissions that gate sidebar items. The server stays the authority; this only drives visibility. */
export type Permission = 'org.admin' | 'project.permissions'

export type OrgInfo = {
  id: string
  name: string
  slug: string
}

export type TopNavMenuKey = 'projects' | 'filters' | 'dashboards' | 'teams'

export type TopNavMenu = {
  key: TopNavMenuKey
  label: string
  /** Target of the "Ver todos" entry and of the top level link. */
  path: string
  /** Recent items. Placeholder until P3 provides the recent endpoint. */
  recent: readonly TopNavLink[]
}

export type TopNavLink = {
  key: string
  label: string
  path: string
}

export type TopNav = {
  menus: readonly TopNavMenu[]
  /** Create button; only for signed-in users. */
  canCreate: boolean
  orgName: string | null
}

export const VIEW_ALL_LABEL = 'Ver todos'

function menu(key: TopNavMenuKey, label: string, path: string): TopNavMenu {
  return { key, label, path, recent: [] }
}

/** Top bar model: dropdown menus, Create button and organization name. Teams is admin only. */
export function buildTopNav(user: Pick<User, 'role'> | null, org: OrgInfo | null): TopNav {
  const menus: TopNavMenu[] = [
    menu('projects', 'Proyectos', '/projects'),
    menu('filters', 'Filtros', '/issues'),
    menu('dashboards', 'Paneles', '/dashboard'),
  ]
  if (user?.role === 'admin') {
    menus.push(menu('teams', 'Equipos', '/teams'))
  }
  return {
    menus,
    canCreate: user !== null,
    orgName: org?.name ?? null,
  }
}

/** Permissions granted to a user. Platform admins get the organization and project administration. */
export function permissionsFor(user: Pick<User, 'role'> | null): ReadonlySet<Permission> {
  const role: Role | null = user?.role ?? null
  if (role === 'admin') {
    return new Set<Permission>(['org.admin', 'project.permissions'])
  }
  return new Set<Permission>()
}

export type SidebarContext =
  | { kind: 'global' }
  | { kind: 'project'; projectId: string }
  | { kind: 'org-admin' }
  | { kind: 'profile' }

export type SidebarItemIcon =
  | 'board'
  | 'backlog'
  | 'sprints'
  | 'calendar'
  | 'permissions'
  | 'settings'
  | 'members'
  | 'invitations'
  | 'profile'

export type SidebarItem = {
  key: string
  label: string
  path: string
  icon: SidebarItemIcon
  permission?: Permission
}

const PROJECT_ITEMS: readonly ((base: string) => SidebarItem)[] = [
  (base) => ({ key: 'board', label: 'Tablero', path: `${base}/board`, icon: 'board' }),
  (base) => ({ key: 'backlog', label: 'Backlog', path: `${base}/backlog`, icon: 'backlog' }),
  (base) => ({ key: 'sprints', label: 'Sprints', path: `${base}/sprints`, icon: 'sprints' }),
  (base) => ({ key: 'calendar', label: 'Calendario', path: `${base}/calendar`, icon: 'calendar' }),
  (base) => ({ key: 'permissions', label: 'Permisos', path: `${base}/permisos`, icon: 'permissions', permission: 'project.permissions' }),
]

const ORG_ADMIN_ITEMS: readonly SidebarItem[] = [
  { key: 'org-settings', label: 'Configuración', path: '/organization/settings', icon: 'settings', permission: 'org.admin' },
  { key: 'org-members', label: 'Miembros', path: '/organization/members', icon: 'members', permission: 'org.admin' },
  { key: 'org-invitations', label: 'Invitaciones', path: '/organization/invitations', icon: 'invitations', permission: 'org.admin' },
]

const PROFILE_ITEMS: readonly SidebarItem[] = [
  { key: 'profile', label: 'General', path: '/profile', icon: 'profile' },
]

/** Pure route-to-context mapping. The sidebar is shown only for contexts other than global. */
export function resolveSidebarContext(pathname: string): SidebarContext {
  const segments = pathname.split('/').filter(Boolean)
  const [first, second] = segments
  if (first === 'projects' && second) {
    return { kind: 'project', projectId: decodeURIComponent(second) }
  }
  if (first === 'organization') return { kind: 'org-admin' }
  if (first === 'profile') return { kind: 'profile' }
  return { kind: 'global' }
}

/** Sidebar items for a context, filtered by the permissions the user holds. Global has no sidebar. */
export function buildSidebar(context: SidebarContext, permissions: ReadonlySet<Permission>): SidebarItem[] {
  let items: readonly SidebarItem[]
  switch (context.kind) {
    case 'global':
      items = []
      break
    case 'project': {
      const base = `/projects/${encodeURIComponent(context.projectId)}`
      items = PROJECT_ITEMS.map((build) => build(base))
      break
    }
    case 'org-admin':
      items = ORG_ADMIN_ITEMS
      break
    case 'profile':
      items = PROFILE_ITEMS
      break
  }
  return items.filter((item) => item.permission === undefined || permissions.has(item.permission))
}

/** True when the pathname belongs to the item (exact or nested). */
export function isSidebarItemActive(item: Pick<SidebarItem, 'path'>, pathname: string): boolean {
  return pathname === item.path || pathname.startsWith(`${item.path}/`)
}
