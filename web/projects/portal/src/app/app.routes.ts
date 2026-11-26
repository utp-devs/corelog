import { Routes } from '@angular/router';

export const routes: Routes = [
  {
    path: '',
    pathMatch: 'full',
    redirectTo: 'login',
  },
  {
    path: 'login',
    loadChildren: () => import('@corelog/features/auth').then((m) => m.AUTH_ROUTES),
  },
  {
    path: '**',
    title: 'Página no encontrada',
    loadComponent: () => import('@corelog/ui/not-found').then((m) => m.NotFound),
  },
];
