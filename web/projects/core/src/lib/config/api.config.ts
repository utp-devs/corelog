import { InjectionToken } from '@angular/core';

/** Base URL of the CoreLog API (same origin in every environment, proxied by nginx or the dev server). */
export const API_URL = new InjectionToken<string>('API_URL', {
  providedIn: 'root',
  factory: () => '/api/v1',
});

/** Name of the application that bootstrapped (portal, workspace or admin). */
export const CORELOG_APP = new InjectionToken<string>('CORELOG_APP', {
  providedIn: 'root',
  factory: () => 'portal',
});
