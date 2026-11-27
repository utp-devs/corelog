import type { ShellNavConfig } from '@corelog/ui/app-shell';

/**
 * Navigation of the workspace shell. Paths inside the workspace are router paths. The organization
 * administration lives in the admin app, so its entries are absolute paths handed over to that app.
 */
export const WORKSPACE_NAV: ShellNavConfig = {
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
        path: '/admin/organization/settings',
        external: true,
        icon: 'settings',
        permission: 'org.admin',
      },
      {
        key: 'org-members',
        label: 'Miembros',
        path: '/admin/organization/members',
        external: true,
        icon: 'members',
        permission: 'org.admin',
      },
      {
        key: 'org-invitations',
        label: 'Invitaciones',
        path: '/admin/organization/invitations',
        external: true,
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
