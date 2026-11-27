import { Routes } from '@angular/router';

import { unsavedChangesGuard } from '@corelog/core';

/**
 * Team pages: the list at the mount point and the team detail at `:id`.
 * Paths are relative to the mount point chosen by the workspace app.
 */
export const TEAMS_ROUTES: Routes = [
  {
    path: '',
    title: 'Equipos',
    canDeactivate: [unsavedChangesGuard],
    loadComponent: () => import('./teams-page/teams-page').then((m) => m.TeamsPage),
  },
  {
    path: ':id',
    title: 'Equipo',
    canDeactivate: [unsavedChangesGuard],
    loadComponent: () => import('./team-page/team-page').then((m) => m.TeamPage),
  },
];
