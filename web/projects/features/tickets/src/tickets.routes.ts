import { Routes } from '@angular/router';

import { unsavedChangesGuard } from '@corelog/core';

/**
 * Issue area, mounted by the workspace app under /workspace/issues.
 * Paths are relative to the mount point. The new-issue form asks before discarding typed values.
 */
export const TICKETS_ROUTES: Routes = [
  {
    path: '',
    title: 'Incidencias',
    loadComponent: () =>
      import('./tickets-list-page/tickets-list-page').then((m) => m.TicketsListPage),
  },
  {
    path: 'new',
    title: 'Nueva incidencia',
    canDeactivate: [unsavedChangesGuard],
    loadComponent: () => import('./new-ticket-page/new-ticket-page').then((m) => m.NewTicketPage),
  },
  {
    path: ':id',
    title: 'Detalle de la incidencia',
    loadComponent: () =>
      import('./ticket-detail-page/ticket-detail-page').then((m) => m.TicketDetailPage),
  },
];
