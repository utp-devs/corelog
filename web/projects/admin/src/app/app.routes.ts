import { Routes } from '@angular/router';

import {
  authGuard,
  externalPath,
  requireOrgAdminGuard,
  requireOrgGuard,
  sameUrl,
} from '@corelog/core';
import { ORG_ADMIN_ROUTES } from '@corelog/features/organizations';

/**
 * Admin routes at /admin/. Work pages, the profile and onboarding belong to other applications and
 * are handed over with a full-page navigation. Organization administration needs an owner or admin
 * role; other members see the 403 page in place of the content.
 */
export const routes: Routes = [
  externalPath('login', '/login'),
  externalPath('profile', '/workspace/profile'),
  externalPath('profile/**', (url) => `/workspace${url}`),
  externalPath('workspace/**', sameUrl),
  externalPath('onboarding', sameUrl),
  {
    path: '',
    canActivate: [authGuard, requireOrgGuard],
    canActivateChild: [authGuard, requireOrgGuard],
    loadComponent: () => import('./shell/admin-shell').then((m) => m.AdminShell),
    children: [
      { path: '', pathMatch: 'full', redirectTo: 'organization/settings' },
      {
        path: 'organization',
        canActivate: [requireOrgAdminGuard],
        canActivateChild: [requireOrgAdminGuard],
        children: [{ path: '', pathMatch: 'full', redirectTo: 'settings' }, ...ORG_ADMIN_ROUTES],
      },
      {
        path: '403',
        title: 'Acceso denegado',
        data: { variant: 403 },
        loadComponent: () => import('@corelog/ui/error-page').then((m) => m.ErrorPage),
      },
      {
        path: '**',
        title: 'Página no encontrada',
        loadComponent: () => import('@corelog/ui/not-found').then((m) => m.NotFound),
      },
    ],
  },
];
