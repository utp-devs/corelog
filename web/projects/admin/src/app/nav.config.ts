import { DEFAULT_SHELL_BRAND, type ShellBrand, type ShellNavConfig } from '@corelog/ui/app-shell';

/**
 * Navigation of the admin shell. Organization administration is local; the work pages (projects,
 * issues, dashboards, teams, people and the profile) live in the workspace app, so their entries are
 * absolute paths handed over to that app.
 */
export const ADMIN_NAV: ShellNavConfig = {
  menus: [
    {
      key: 'projects',
      label: 'Proyectos',
      path: '/workspace/projects',
      external: true,
      linkBase: '/workspace',
    },
    {
      key: 'filters',
      label: 'Filtros',
      path: '/workspace/issues',
      external: true,
      linkBase: '/workspace',
    },
    {
      key: 'dashboards',
      label: 'Paneles',
      path: '/workspace/dashboard',
      external: true,
      linkBase: '/workspace',
    },
    {
      key: 'teams',
      label: 'Equipos',
      path: '/workspace/teams',
      external: true,
      linkBase: '/workspace',
      extras: [{ key: 'people', label: 'Ver personas', path: '/workspace/people', external: true }],
    },
  ],
  sidebars: {
    project: [
      {
        key: 'board',
        label: 'Tablero',
        path: '/workspace/projects/:projectId/board',
        external: true,
        icon: 'board',
      },
      {
        key: 'backlog',
        label: 'Backlog',
        path: '/workspace/projects/:projectId/backlog',
        external: true,
        icon: 'backlog',
      },
      {
        key: 'sprints',
        label: 'Sprints',
        path: '/workspace/projects/:projectId/sprints',
        external: true,
        icon: 'sprints',
      },
      {
        key: 'calendar',
        label: 'Calendario',
        path: '/workspace/projects/:projectId/calendar',
        external: true,
        icon: 'calendar',
      },
      {
        key: 'permissions',
        label: 'Permisos',
        path: '/workspace/projects/:projectId/permisos',
        external: true,
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
    profile: [
      {
        key: 'profile',
        label: 'General',
        path: '/workspace/profile',
        external: true,
        icon: 'profile',
        exact: true,
      },
      {
        key: 'security',
        label: 'Seguridad',
        path: '/workspace/profile/security',
        external: true,
        icon: 'security',
      },
      {
        key: 'notifications',
        label: 'Notificaciones',
        path: '/workspace/profile/notifications',
        external: true,
        icon: 'bell',
      },
      {
        key: 'tokens',
        label: 'Tokens',
        path: '/workspace/profile/tokens',
        external: true,
        icon: 'tokens',
      },
      {
        key: 'appearance',
        label: 'Apariencia',
        path: '/workspace/profile/appearance',
        external: true,
        icon: 'appearance',
      },
    ],
  },
  sidebarTitles: {
    project: 'Proyecto',
    'org-admin': 'Organización',
    profile: 'Mi cuenta',
  },
};

/** Logo of the admin shell links to the workspace home. */
export const ADMIN_BRAND: ShellBrand = {
  ...DEFAULT_SHELL_BRAND,
  homePath: '/workspace/issues',
  external: true,
};

/**
 * Addresses of the help menu. An entry is listed only when its address is set; set them when the
 * documentation, release notes and support pages exist.
 */
export const HELP_LINKS: {
  readonly docsUrl: string | null;
  readonly whatsNewUrl: string | null;
  readonly supportUrl: string | null;
} = { docsUrl: null, whatsNewUrl: null, supportUrl: null };
