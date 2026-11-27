import { Routes } from '@angular/router';

/**
 * People directory and person cards, mounted by the workspace app under /workspace/people.
 * Paths are relative to the mount point.
 */
export const PEOPLE_ROUTES: Routes = [
  {
    path: '',
    title: 'Personas',
    loadComponent: () => import('./people-page/people-page').then((m) => m.PeoplePage),
  },
  {
    path: ':id',
    title: 'Persona',
    loadComponent: () => import('./person-page/person-page').then((m) => m.PersonPage),
  },
];
