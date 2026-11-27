import { Routes } from '@angular/router';
import { unsavedChangesGuard } from '@corelog/core';

/**
 * Settings of a project (`/projects/:key/settings/...`): one child per section, inside the settings
 * shell. Only the sections that exist today are listed.
 */
export const PROJECT_SETTINGS_ROUTES: Routes = [
  {
    path: 'settings',
    title: 'Configuración',
    loadComponent: () => import('./project-settings-shell').then((m) => m.ProjectSettingsShell),
    children: [
      { path: '', pathMatch: 'full', redirectTo: 'details' },
      {
        path: 'details',
        title: 'Detalles del proyecto',
        canDeactivate: [unsavedChangesGuard],
        loadComponent: () =>
          import('./details/project-details-page').then((m) => m.ProjectDetailsPage),
      },
      {
        path: 'access',
        title: 'Acceso al proyecto',
        loadComponent: () =>
          import('./access/project-access-page').then((m) => m.ProjectAccessPage),
      },
      {
        path: 'roles',
        title: 'Roles del proyecto',
        loadComponent: () => import('./roles/project-roles-page').then((m) => m.ProjectRolesPage),
      },
      {
        path: 'permissions',
        title: 'Permisos del proyecto',
        canDeactivate: [unsavedChangesGuard],
        loadComponent: () =>
          import('./permissions/project-permissions-matrix-page').then(
            (m) => m.ProjectPermissionsMatrixPage,
          ),
      },
      {
        path: 'danger',
        title: 'Zona de peligro',
        loadComponent: () =>
          import('./danger/project-danger-page').then((m) => m.ProjectDangerPage),
      },
    ],
  },
];
