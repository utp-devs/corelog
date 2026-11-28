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
    // The project sidebar is built by the shell from the project in view (template and permissions), so no static items.
    project: [],
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
        key: 'org-catalogs',
        label: 'Incidencias',
        path: '/organization/catalogs',
        icon: 'issue',
        permission: 'org.admin',
      },
      {
        key: 'org-custom-fields',
        label: 'Campos',
        path: '/organization/custom-fields',
        icon: 'issue',
        permission: 'org.admin',
      },
      {
        key: 'org-statuses',
        label: 'Estados',
        path: '/organization/statuses',
        icon: 'workflow',
        permission: 'org.admin',
      },
      {
        key: 'org-invitations',
        label: 'Invitaciones',
        path: '/organization/invitations',
        icon: 'invitations',
        permission: 'org.admin',
      },
      {
        key: 'org-audit',
        label: 'Auditoría',
        path: '/organization/audit',
        icon: 'security',
        permission: 'org.admin',
      },
      {
        key: 'org-platform-audit',
        label: 'Auditoría de la plataforma',
        path: '/organization/platform-audit',
        icon: 'security',
        permission: 'platform.admin',
      },
      {
        key: 'org-legal',
        label: 'Documentos legales',
        path: '/organization/legal',
        icon: 'security',
        permission: 'platform.admin',
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
        key: 'appearance',
        label: 'Apariencia',
        path: '/workspace/profile/appearance',
        external: true,
        icon: 'appearance',
      },
      {
        key: 'privacy',
        label: 'Privacidad',
        path: '/workspace/profile/privacy',
        external: true,
        icon: 'security',
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
