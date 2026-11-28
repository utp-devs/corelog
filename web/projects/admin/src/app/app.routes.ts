import { Routes } from '@angular/router';

import {
  authGuard,
  externalPath,
  requireOrgAdminGuard,
  requireOrgGuard,
  requirePlatformAdminGuard,
  sameUrl,
} from '@corelog/core';
import { ORG_AUDIT_ROUTES, PLATFORM_AUDIT_ROUTES } from '@corelog/features/audit';
import { ORG_WEBHOOKS_ROUTES } from '@corelog/features/integrations';
import { ORG_NOTIFICATION_ROUTES } from '@corelog/features/notification-settings';
import {
  ORG_ADMIN_ROUTES,
  ORG_OVERVIEW_ROUTES,
  PLATFORM_ADMIN_ROUTES,
} from '@corelog/features/organizations';

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
      { path: '', pathMatch: 'full', redirectTo: 'organization/overview' },
      {
        path: 'organization',
        canActivate: [requireOrgAdminGuard],
        canActivateChild: [requireOrgAdminGuard],
        children: [
          { path: '', pathMatch: 'full', redirectTo: 'overview' },
          ...ORG_OVERVIEW_ROUTES,
          ...ORG_ADMIN_ROUTES,
          ...ORG_NOTIFICATION_ROUTES,
          ...PLATFORM_ADMIN_ROUTES,
          { path: 'audit', children: ORG_AUDIT_ROUTES },
          { path: 'webhooks', children: ORG_WEBHOOKS_ROUTES },
          {
            path: 'platform-audit',
            canActivate: [requirePlatformAdminGuard],
            children: PLATFORM_AUDIT_ROUTES,
          },
        ],
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
