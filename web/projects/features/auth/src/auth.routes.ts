import { Routes } from '@angular/router';

/**
 * Public sign-in area: sign-in, account creation and invitation preview.
 * Paths are relative to the mount point chosen by the portal app.
 */
export const AUTH_ROUTES: Routes = [
  {
    path: 'login',
    title: 'Iniciar sesión',
    loadComponent: () => import('./login-page/login-page').then((m) => m.LoginPage),
  },
  {
    path: 'register',
    title: 'Crear cuenta',
    loadComponent: () => import('./register-page/register-page').then((m) => m.RegisterPage),
  },
  {
    path: 'invite/:token',
    title: 'Invitación',
    loadComponent: () =>
      import('./invite-accept-page/invite-accept-page').then((m) => m.InviteAcceptPage),
  },
];
