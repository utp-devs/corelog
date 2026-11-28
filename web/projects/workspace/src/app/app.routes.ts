import { Routes } from '@angular/router';

import { authGuard, externalPath, requireOrgGuard, sameUrl } from '@corelog/core';
import { CALENDAR_ROUTES } from '@corelog/features/calendar';
import { DASHBOARDS_ROUTES, DASHBOARD_ROUTES } from '@corelog/features/dashboard';
import { FILTERS_ROUTES } from '@corelog/features/filters';
import { NOTES_ROUTES } from '@corelog/features/notes';
import { PEOPLE_ROUTES } from '@corelog/features/people';
import { PROFILE_ROUTES } from '@corelog/features/profile';
import { PROJECT_PLANNING_ROUTES } from '@corelog/features/project-planning';
import { ROADMAP_ROUTES } from '@corelog/features/roadmap';
import { PROJECTS_ROUTES, PROJECT_DETAIL_CHILDREN } from '@corelog/features/projects';
import { TEAMS_ROUTES } from '@corelog/features/teams';
import { ISSUE_BROWSE_ROUTES, ISSUES_ROUTES, TIMESHEET_ROUTES } from '@corelog/features/issues';

const [projectList, projectNew, projectDetail] = PROJECTS_ROUTES;

/**
 * Project list at /projects and the project workspace at /projects/:key. The planning screens are
 * appended to the project children; an unknown child opens the board.
 */
const PROJECT_ROUTES: Routes = [
  projectList,
  projectNew,
  {
    ...projectDetail,
    children: [
      ...PROJECT_DETAIL_CHILDREN,
      ...PROJECT_PLANNING_ROUTES,
      ...ROADMAP_ROUTES,
      { path: '**', redirectTo: 'board' },
    ],
  },
];

/**
 * Workspace routes at /workspace/. Paths that belong to the portal or the admin app are handed over
 * with a full-page navigation. Everything inside the shell needs a session and an active organization.
 */
export const routes: Routes = [
  externalPath('login', '/login'),
  externalPath('onboarding', sameUrl),
  externalPath('admin/**', sameUrl),
  {
    path: '',
    canActivate: [authGuard, requireOrgGuard],
    canActivateChild: [authGuard, requireOrgGuard],
    loadComponent: () => import('./shell/workspace-shell').then((m) => m.WorkspaceShell),
    children: [
      { path: '', pathMatch: 'full', redirectTo: 'issues' },
      { path: 'issues', children: ISSUES_ROUTES },
      { path: 'browse', children: ISSUE_BROWSE_ROUTES },
      { path: 'timesheet', children: TIMESHEET_ROUTES },
      { path: 'filters', children: FILTERS_ROUTES },
      { path: 'profile', children: PROFILE_ROUTES },
      { path: 'people', children: PEOPLE_ROUTES },
      { path: 'projects', children: PROJECT_ROUTES },
      { path: 'dashboard', children: DASHBOARD_ROUTES },
      { path: 'dashboards', children: DASHBOARDS_ROUTES },
      { path: 'me/dashboard', pathMatch: 'full', redirectTo: 'dashboard' },
      { path: 'teams', children: TEAMS_ROUTES },
      { path: 'notes', children: NOTES_ROUTES },
      { path: 'calendar', children: CALENDAR_ROUTES },
      {
        path: '**',
        title: 'Página no encontrada',
        loadComponent: () => import('@corelog/ui/not-found').then((m) => m.NotFound),
      },
    ],
  },
];
