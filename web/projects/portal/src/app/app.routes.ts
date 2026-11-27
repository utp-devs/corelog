import { Routes } from '@angular/router';

import { authGuard, guestGuard } from '@corelog/core';
import { AUTH_ROUTES } from '@corelog/features/auth';
import { ONBOARDING_ROUTES } from '@corelog/features/organizations';
import { landingGuard } from './routing/landing.guard';

/** Pages that only signed-out users may open. The invitation preview is public. */
const GUEST_ONLY_PATHS = new Set(['login', 'register']);

/**
 * Portal routes at the origin root: sign-in, registration, invitation preview (public) and the
 * onboarding wizard (signed in, outside the app shell). Everything else is a not-found page.
 */
export const routes: Routes = [
  { path: '', pathMatch: 'full', canActivate: [landingGuard], children: [] },
  ...AUTH_ROUTES.map((route) =>
    GUEST_ONLY_PATHS.has(route.path ?? '') ? { ...route, canActivate: [guestGuard] } : route,
  ),
  {
    path: 'onboarding',
    canActivate: [authGuard],
    canActivateChild: [authGuard],
    children: ONBOARDING_ROUTES,
  },
  {
    path: '**',
    title: 'Página no encontrada',
    loadComponent: () => import('@corelog/ui/not-found').then((m) => m.NotFound),
  },
];
