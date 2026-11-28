import { Routes } from '@angular/router';

/**
 * Roadmap of a project. It is a child of the project workspace, which resolves the project from the
 * `:key` route parameter, so the path is relative to `/projects/:key`.
 */
export const ROADMAP_ROUTES: Routes = [
  {
    path: 'roadmap',
    title: 'Hoja de ruta',
    loadComponent: () => import('./roadmap-page/roadmap-page').then((m) => m.RoadmapPage),
  },
];
