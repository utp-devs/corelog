import type { ShellNavConfig } from '@corelog/ui/app-shell';

/**
 * Navigation of the workspace shell. Paths inside the workspace are router paths. The organization
 * administration lives in the admin app, so its entries are absolute paths handed over to that app.
 */
export const WORKSPACE_NAV: ShellNavConfig = {
  menus: [
    { key: 'projects', label: 'Proyectos', path: '/projects' },
    { key: 'filters', label: 'Filtros', path: '/issues' },
    {
      key: 'dashboards',
      label: 'Paneles',
      path: '/dashboard',
      extras: [
        { key: 'notes', label: 'Mis apuntes', path: '/notes' },
        { key: 'timesheet', label: 'Mi hoja de horas', path: '/timesheet' },
      ],
    },
    {
      key: 'teams',
      label: 'Equipos',
      path: '/teams',
      extras: [{ key: 'people', label: 'Ver personas', path: '/people' }],
    },
  ],
  sidebars: {
    // The project sidebar is built by the shell from the project in view (template and permissions), so no static items.
    project: [],
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
      {
        key: 'org-legal',
        label: 'Documentos legales',
        path: '/admin/organization/legal',
        icon: 'security',
        permission: 'platform.admin',
      },
    ],
    profile: [
      { key: 'profile', label: 'General', path: '/profile', icon: 'profile', exact: true },
      { key: 'security', label: 'Seguridad', path: '/profile/security', icon: 'security' },
      {
        key: 'notifications',
        label: 'Notificaciones',
        path: '/profile/notifications',
        icon: 'bell',
      },
      { key: 'appearance', label: 'Apariencia', path: '/profile/appearance', icon: 'appearance' },
      { key: 'privacy', label: 'Privacidad', path: '/profile/privacy', icon: 'security' },
    ],
  },
  sidebarTitles: {
    project: 'Proyecto',
    'org-admin': 'Organización',
    profile: 'Mi cuenta',
  },
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
