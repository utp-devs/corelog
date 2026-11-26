import type { Role, User } from '@/core/auth/domain/user'
import { canManageOrg, type OrgRole } from '@/core/organizations/domain/organization'

/** Permissions that gate sidebar items. The server stays the authority; this only drives visibility. */
export type Permission = 'org.admin' | 'project.permissions'

export type OrgInfo = {
  id: string
  name: string
  slug: string
  /** Caller role in the organization; owners and admins get the organization administration items. */
  role?: OrgRole | null
}

export type TopNavMenuKey = 'projects' | 'filters' | 'dashboards' | 'teams'

export type TopNavMenu = {
  key: TopNavMenuKey
  label: string
  /** Target of the "Ver todos" entry and of the top level link. */
  path: string
  /** Recent items. Placeholder until the recent-items endpoint exists. */
  recent: readonly TopNavLink[]
  /** Extra entries listed after "Ver todos" (for example "Ver personas" under Equipos). */
  extras?: readonly TopNavLink[]
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

function menu(key: TopNavMenuKey, label: string, path: string, extras?: readonly TopNavLink[]): TopNavMenu {
  return extras ? { key, label, path, recent: [], extras } : { key, label, path, recent: [] }
}

/** Top bar model: dropdown menus, Create button and organization name. Teams is open to every organization member. */
export function buildTopNav(user: Pick<User, 'role'> | null, org: OrgInfo | null): TopNav {
  const menus: TopNavMenu[] = [
    menu('projects', 'Proyectos', '/projects'),
    menu('filters', 'Filtros', '/issues'),
    menu('dashboards', 'Paneles', '/dashboard'),
    menu('teams', 'Equipos', '/teams', [{ key: 'people', label: 'Ver personas', path: '/people' }]),
  ]
  return {
    menus,
    canCreate: user !== null,
    orgName: org?.name ?? null,
  }
}

/**
 * Permissions granted to a user. Platform admins and owners/admins of the active organization get the
 * organization administration; platform admins also get project permissions.
 */
export function permissionsFor(user: Pick<User, 'role'> | null, orgRole?: OrgRole | null): ReadonlySet<Permission> {
  const role: Role | null = user?.role ?? null
  const granted = new Set<Permission>()
  if (canManageOrg(orgRole)) granted.add('org.admin')
  if (role === 'admin') {
    granted.add('org.admin')
    granted.add('project.permissions')
  }
  return granted
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
