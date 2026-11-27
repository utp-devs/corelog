import { hasPermission, type OrgRole, type Permission } from '@corelog/core';

import type { ShellIconName } from './shell-icon';

/** Label of the "view all" entry at the end of each top menu. */
export const VIEW_ALL_LABEL = 'Ver todos';

/** Sidebar contexts that have their own item list. The global context has no sidebar. */
export type SidebarContextKind = 'project' | 'org-admin' | 'profile';

export type SidebarContext =
  | { readonly kind: 'global' }
  | { readonly kind: 'project'; readonly projectId: string }
  | { readonly kind: 'org-admin' }
  | { readonly kind: 'profile' };

/**
 * One sidebar entry. `path` is absolute. In the project context the placeholder `:projectId` is
 * replaced with the encoded id of the project in view. Items with `permission` are listed only
 * when the user holds it.
 */
export interface NavItem {
  readonly key: string;
  readonly label: string;
  readonly path: string;
  readonly icon: ShellIconName;
  readonly permission?: Permission;
  /** Another application of the origin: opened with a full-page navigation. */
  readonly external?: boolean;
}

/** Entry listed under a top menu, after "Ver todos". */
export interface NavLink {
  readonly key: string;
  readonly label: string;
  readonly path: string;
  /** Another application of the origin: opened with a full-page navigation. */
  readonly external?: boolean;
}

/** Top bar dropdown. `path` is the target of "Ver todos" and of the drawer link. */
export interface NavMenu {
  readonly key: string;
  readonly label: string;
  readonly path: string;
  /** Another application of the origin: "Ver todos" opens it with a full-page navigation. */
  readonly external?: boolean;
  readonly extras?: readonly NavLink[];
}

/** Organization shown in the shell. `role` is the caller role in that organization. */
export interface ShellOrg {
  readonly id: string;
  readonly name: string;
  readonly slug: string;
  readonly role?: OrgRole | null;
}

/** Shape of the shell navigation. Apps pass their own copy to `AppShell` through the `navConfig` input. */
export interface ShellNavConfig {
  readonly menus: readonly NavMenu[];
  readonly sidebars: Readonly<Record<SidebarContextKind, readonly NavItem[]>>;
  readonly sidebarTitles: Readonly<Record<SidebarContextKind, string>>;
}

export interface TopNav {
  readonly menus: readonly NavMenu[];
  readonly orgName: string | null;
}

export const DEFAULT_SHELL_NAV_CONFIG: ShellNavConfig = {
  menus: [
    { key: 'projects', label: 'Proyectos', path: '/projects' },
    { key: 'filters', label: 'Filtros', path: '/issues' },
    { key: 'dashboards', label: 'Paneles', path: '/dashboard' },
    {
      key: 'teams',
      label: 'Equipos',
      path: '/teams',
      extras: [{ key: 'people', label: 'Ver personas', path: '/people' }],
    },
  ],
  sidebars: {
    project: [
      { key: 'board', label: 'Tablero', path: '/projects/:projectId/board', icon: 'board' },
      { key: 'backlog', label: 'Backlog', path: '/projects/:projectId/backlog', icon: 'backlog' },
      { key: 'sprints', label: 'Sprints', path: '/projects/:projectId/sprints', icon: 'sprints' },
      {
        key: 'calendar',
        label: 'Calendario',
        path: '/projects/:projectId/calendar',
        icon: 'calendar',
      },
      {
        key: 'permissions',
        label: 'Permisos',
        path: '/projects/:projectId/permisos',
        icon: 'permissions',
        permission: 'project.permissions',
      },
    ],
    'org-admin': [
      {
        key: 'org-settings',
        label: 'Configuración',
        path: '/organization/settings',
        icon: 'settings',
        permission: 'org.admin',
      },
      {
        key: 'org-members',
        label: 'Miembros',
        path: '/organization/members',
        icon: 'members',
        permission: 'org.admin',
      },
      {
        key: 'org-invitations',
        label: 'Invitaciones',
        path: '/organization/invitations',
        icon: 'invitations',
        permission: 'org.admin',
      },
    ],
    profile: [{ key: 'profile', label: 'General', path: '/profile', icon: 'profile' }],
  },
  sidebarTitles: {
    project: 'Proyecto',
    'org-admin': 'Organización',
    profile: 'Mi cuenta',
  },
};

/** Top bar model: the configured menus and the organization name. */
export function buildTopNav(
  org: ShellOrg | null,
  config: ShellNavConfig = DEFAULT_SHELL_NAV_CONFIG,
): TopNav {
  return { menus: config.menus, orgName: org?.name ?? null };
}

/** Pure route-to-context mapping. The sidebar is shown only for contexts other than global. */
export function resolveSidebarContext(pathname: string): SidebarContext {
  const [first, second] = pathname.split('/').filter(Boolean);
  if (first === 'projects' && second) {
    return { kind: 'project', projectId: safeDecode(second) };
  }
  if (first === 'organization') return { kind: 'org-admin' };
  if (first === 'profile') return { kind: 'profile' };
  return { kind: 'global' };
}

/** Sidebar items for a context, filtered by the granted permissions and with project placeholders resolved. */
export function buildSidebar(
  context: SidebarContext,
  granted: ReadonlySet<Permission>,
  config: ShellNavConfig = DEFAULT_SHELL_NAV_CONFIG,
): NavItem[] {
  if (context.kind === 'global') return [];
  const projectId = context.kind === 'project' ? encodeURIComponent(context.projectId) : null;
  return config.sidebars[context.kind]
    .filter((item) => hasPermission(granted, item.permission))
    .map((item) =>
      projectId === null ? item : { ...item, path: item.path.replace(':projectId', projectId) },
    );
}

/** Title of the sidebar for a context. Empty for the global context. */
export function sidebarTitleOf(
  context: SidebarContext,
  config: ShellNavConfig = DEFAULT_SHELL_NAV_CONFIG,
): string {
  return context.kind === 'global' ? '' : config.sidebarTitles[context.kind];
}

/** True when the pathname belongs to the item (exact or nested). */
export function isSidebarItemActive(item: Pick<NavItem, 'path'>, pathname: string): boolean {
  return pathname === item.path || pathname.startsWith(`${item.path}/`);
}

function safeDecode(segment: string): string {
  try {
    return decodeURIComponent(segment);
  } catch {
    return segment;
  }
}
