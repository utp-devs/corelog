import { Routes } from '@angular/router';

import { requirePlatformAdminGuard, unsavedChangesGuard } from '@corelog/core';

/**
 * Organization administration, mounted by the admin app under /admin/organization. The app guards the
 * mount point with requireOrgAdmin; permission-specific states are rendered by the pages themselves.
 */
export const ORG_ADMIN_ROUTES: Routes = [
  {
    path: 'settings',
    title: 'Configuración de la organización',
    canDeactivate: [unsavedChangesGuard],
    loadComponent: () => import('./settings/org-settings-page').then((m) => m.OrgSettingsPage),
  },
  {
    path: 'members',
    title: 'Miembros',
    loadComponent: () => import('./members/org-members-page').then((m) => m.OrgMembersPage),
  },
  {
    path: 'catalogs',
    title: 'Tipos, prioridades y resoluciones',
    loadComponent: () => import('./catalogs/org-catalogs-page').then((m) => m.OrgCatalogsPage),
  },
  {
    path: 'custom-fields',
    title: 'Campos personalizados',
    loadComponent: () =>
      import('./custom-fields/org-custom-fields-page').then((m) => m.OrgCustomFieldsPage),
  },
  {
    path: 'statuses',
    title: 'Estados de incidencias',
    loadComponent: () => import('./statuses/org-statuses-page').then((m) => m.OrgStatusesPage),
  },
  {
    path: 'invitations',
    title: 'Invitaciones',
    loadComponent: () =>
      import('./invitations/org-invitations-page').then((m) => m.OrgInvitationsPage),
  },
  {
    path: 'legal',
    title: 'Documentos legales',
    canActivate: [requirePlatformAdminGuard],
    loadComponent: () => import('./legal/org-legal-page').then((m) => m.OrgLegalPage),
  },
];

/** Onboarding wizard, mounted by the portal at /onboarding. Authenticated, outside the app shell. */
export const ONBOARDING_ROUTES: Routes = [
  {
    path: '',
    pathMatch: 'full',
    title: 'Asistente de alta',
    loadComponent: () => import('./onboarding/onboarding-page').then((m) => m.OnboardingPage),
  },
];
