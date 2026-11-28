import { Routes } from '@angular/router';

/** Saved filters manager, mounted by the workspace app under /workspace/filters. */
export const FILTERS_ROUTES: Routes = [
  {
    path: '',
    title: 'Filtros',
    loadComponent: () => import('./filters-page/filters-page').then((m) => m.FiltersPage),
  },
];
