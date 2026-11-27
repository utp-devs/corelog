import { Routes } from '@angular/router';

import { unsavedChangesGuard } from '@corelog/core';

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
    path: 'invitations',
    title: 'Invitaciones',
    loadComponent: () =>
      import('./invitations/org-invitations-page').then((m) => m.OrgInvitationsPage),
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
