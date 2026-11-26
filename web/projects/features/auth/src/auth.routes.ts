import { Routes } from '@angular/router';

import { guestGuard } from '@corelog/core';

/** Routes of the sign-in area, mounted by the portal at /login. Placeholder for now. */
export const AUTH_ROUTES: Routes = [
  {
    path: '',
    pathMatch: 'full',
    title: 'Iniciar sesión',
    canActivate: [guestGuard],
    loadComponent: () => import('./login-page/login-page').then((m) => m.LoginPage),
  },
];
