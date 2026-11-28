import { Routes } from '@angular/router';

/**
 * Planning screens of a project: board, backlog, sprints and the report of a sprint. They are children of
 * the project workspace, which provides the project id through its `:id` route parameter.
 */
export const PROJECT_PLANNING_ROUTES: Routes = [
  {
    path: 'board',
    title: 'Tablero',
    loadComponent: () => import('./board/project-board-page').then((m) => m.ProjectBoardPage),
  },
  {
    path: 'backlog',
    title: 'Backlog',
    loadComponent: () => import('./backlog/project-backlog-page').then((m) => m.ProjectBacklogPage),
  },
  {
    path: 'sprints',
    title: 'Sprints',
    loadComponent: () => import('./sprints/project-sprints-page').then((m) => m.ProjectSprintsPage),
  },
  {
    path: 'sprints/:id/report',
    title: 'Informe del sprint',
    loadComponent: () => import('./sprints/sprint-report-page').then((m) => m.SprintReportPage),
  },
];
