import { DEFAULT_SHELL_BRAND, type ShellBrand, type ShellNavConfig } from '@corelog/ui/app-shell';

/**
 * Navigation of the admin shell. Organization administration is local; the work pages (projects,
 * issues, dashboards, teams, people and the profile) live in the workspace app, so their entries are
 * absolute paths handed over to that app.
 */
export const ADMIN_NAV: ShellNavConfig = {
  menus: [
    { key: 'projects', label: 'Proyectos', path: '/workspace/projects', external: true },
    { key: 'filters', label: 'Filtros', path: '/workspace/issues', external: true },
    { key: 'dashboards', label: 'Paneles', path: '/workspace/dashboard', external: true },
    {
      key: 'teams',
      label: 'Equipos',
      path: '/workspace/teams',
      external: true,
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
