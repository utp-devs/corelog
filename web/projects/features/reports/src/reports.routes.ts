import { Routes } from '@angular/router';

/**
 * Reports of a project, children of the project workspace: the hub at `reports` and one report at
 * `reports/:kind`. The report parameters travel in the query string, so a report URL can be shared.
 */
export const REPORTS_ROUTES: Routes = [
  {
    path: '',
    title: 'Reportes',
    loadComponent: () =>
      import('./reports-hub-page/reports-hub-page').then((m) => m.ReportsHubPage),
  },
  {
    path: ':kind',
    title: 'Reporte',
    loadComponent: () => import('./report-page/report-page').then((m) => m.ReportPage),
  },
];
