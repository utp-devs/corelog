import { Routes } from '@angular/router';

/**
 * Audit log pages. The organization log is mounted by the admin app under /organization/audit and the platform
 * log under /audit; the route data tells the page which one it shows.
 */
export const ORG_AUDIT_ROUTES: Routes = [
  {
    path: '',
    title: 'Auditoría',
    data: { scope: 'organization' },
    loadComponent: () => import('./audit-page/audit-page').then((m) => m.AuditPage),
  },
];

export const PLATFORM_AUDIT_ROUTES: Routes = [
  {
    path: '',
    title: 'Auditoría de la plataforma',
    data: { scope: 'platform' },
    loadComponent: () => import('./audit-page/audit-page').then((m) => m.AuditPage),
  },
];
