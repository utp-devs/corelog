import { Routes } from '@angular/router';
import { PROJECT_ROUTE_PARAM, projectRouteResolver } from '@corelog/core';

/**
 * Children of the project workspace (`/projects/:key`). The planning sections (board, backlog and
 * sprints) are appended by the application router; the default child opens the board.
 */
export const PROJECT_DETAIL_CHILDREN: Routes = [
  {
    path: '',
    pathMatch: 'full',
    redirectTo: 'board',
  },
  {
    path: 'calendar',
    title: 'Calendario',
    loadComponent: () =>
      import('./project-calendar/project-calendar-page').then((m) => m.ProjectCalendarPage),
  },
  {
    path: 'permisos',
    title: 'Permisos',
    loadComponent: () =>
      import('./project-permissions/project-permissions-page').then(
        (m) => m.ProjectPermissionsPage,
      ),
  },
];

/** Project list and project workspace, mounted by the application at /projects. The workspace route
 * takes the project key (or UUID) and resolves it into the current project. */
export const PROJECTS_ROUTES: Routes = [
  {
    path: '',
    title: 'Proyectos',
    loadComponent: () => import('./projects-page/projects-page').then((m) => m.ProjectsPage),
  },
  {
    path: `:${PROJECT_ROUTE_PARAM}`,
    resolve: { project: projectRouteResolver },
    loadComponent: () =>
      import('./project-workspace/project-workspace').then((m) => m.ProjectWorkspace),
    children: PROJECT_DETAIL_CHILDREN,
  },
];
