import { Routes } from '@angular/router';
import { unsavedChangesGuard } from '@corelog/core';

/**
 * Personal dashboard, mounted by the workspace app under /workspace/dashboard. Paths are relative to
 * the mount point chosen by the app.
 */
export const DASHBOARD_ROUTES: Routes = [
  {
    path: '',
    title: 'Mi resumen',
    canDeactivate: [unsavedChangesGuard],
    loadComponent: () => import('./dashboard-page/dashboard-page').then((m) => m.DashboardPage),
  },
];

/** Manager of dashboards and any single dashboard, mounted under /workspace/dashboards. */
export const DASHBOARDS_ROUTES: Routes = [
  {
    path: '',
    title: 'Paneles',
    loadComponent: () => import('./dashboards-page/dashboards-page').then((m) => m.DashboardsPage),
  },
  {
    path: ':id',
    title: 'Panel',
    canDeactivate: [unsavedChangesGuard],
    loadComponent: () => import('./dashboard-page/dashboard-page').then((m) => m.DashboardPage),
  },
];
