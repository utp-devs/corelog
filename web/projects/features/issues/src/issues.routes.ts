import { Routes } from '@angular/router';

import { unsavedChangesGuard } from '@corelog/core';

/**
 * Issue list and creation, mounted by the workspace app under /workspace/issues.
 * Paths are relative to the mount point. The new-issue form asks before discarding typed values.
 * The old detail address /issues/:id keeps working: it redirects to /browse/:id.
 */
export const ISSUES_ROUTES: Routes = [
  {
    path: '',
    title: 'Incidencias',
    loadComponent: () =>
      import('./issues-list-page/issues-list-page').then((m) => m.IssuesListPage),
  },
  {
    path: 'new',
    title: 'Nueva incidencia',
    canDeactivate: [unsavedChangesGuard],
    loadComponent: () => import('./new-issue-page/new-issue-page').then((m) => m.NewIssuePage),
  },
  { path: ':id', redirectTo: '/browse/:id' },
];

/**
 * One issue, mounted by the workspace app under /workspace/browse. The segment is the issue key
 * (CORE-12) or its UUID.
 */
export const ISSUE_BROWSE_ROUTES: Routes = [
  {
    path: ':issueKey',
    title: 'Detalle de la incidencia',
    loadComponent: () =>
      import('./issue-detail-page/issue-detail-page').then((m) => m.IssueDetailPage),
  },
];

/** Personal timesheet, mounted by the workspace app under /workspace/timesheet. */
export const TIMESHEET_ROUTES: Routes = [
  {
    path: '',
    title: 'Mi hoja de horas',
    loadComponent: () => import('./timesheet-page/timesheet-page').then((m) => m.TimesheetPage),
  },
];
