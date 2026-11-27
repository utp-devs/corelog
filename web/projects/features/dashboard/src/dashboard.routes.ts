import { Routes } from '@angular/router';

/**
 * Personal dashboard, mounted by the workspace app under /workspace/dashboard.
 * Paths are relative to the mount point chosen by the app.
 */
export const DASHBOARD_ROUTES: Routes = [
  {
    path: '',
    title: 'Paneles',
    loadComponent: () =>
      import('./my-dashboard-page/my-dashboard-page').then((m) => m.MyDashboardPage),
  },
];
