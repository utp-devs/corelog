import { Routes } from '@angular/router';

/** Landing of the organization administration, mounted under /organization. */
export const ORG_OVERVIEW_ROUTES: Routes = [
  {
    path: 'overview',
    title: 'Resumen de la organización',
    loadComponent: () => import('./overview/org-overview-page').then((m) => m.OrgOverviewPage),
  },
];
