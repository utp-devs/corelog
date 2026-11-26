import { DOCUMENT } from '@angular/common';
import { InjectionToken, inject } from '@angular/core';

/** Login page of the portal (same origin, outside the workspace and admin apps). */
export const LOGIN_URL = new InjectionToken<string>('LOGIN_URL', {
  providedIn: 'root',
  factory: () => '/login',
});

/** Page a signed-in user is sent to when visiting the login page. */
export const HOME_URL = new InjectionToken<string>('HOME_URL', {
  providedIn: 'root',
  factory: () => '/workspace/',
});

/**
 * Full-page navigation. Used for cross-app moves (portal, workspace, admin are separate
 * Angular apps under one origin), so the router cannot be used.
 */
export const LOCATION_ASSIGN = new InjectionToken<(url: string) => void>('LOCATION_ASSIGN', {
  providedIn: 'root',
  factory: () => {
    const document = inject(DOCUMENT);
    return (url: string) => document.defaultView?.location.assign(url);
  },
});
