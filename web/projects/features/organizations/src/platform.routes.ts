import { Routes } from '@angular/router';

import { requirePlatformAdminGuard } from '@corelog/core';

/** Platform administration pages, mounted under /organization. Platform administrators only. */
export const PLATFORM_ADMIN_ROUTES: Routes = [
  {
    path: 'email-outbox',
    title: 'Cola de correo',
    canActivate: [requirePlatformAdminGuard],
    loadComponent: () => import('./platform/email-outbox-page').then((m) => m.EmailOutboxPage),
  },
  {
    path: 'jobs',
    title: 'Trabajos programados',
    canActivate: [requirePlatformAdminGuard],
    loadComponent: () => import('./platform/scheduled-jobs-page').then((m) => m.ScheduledJobsPage),
  },
];
