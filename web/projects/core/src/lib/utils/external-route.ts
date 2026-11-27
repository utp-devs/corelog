import { inject } from '@angular/core';
import { CanActivateFn, Route } from '@angular/router';

import { LOCATION_ASSIGN } from '../config/navigation';

/**
 * Sends the browser to another application of the origin with a full-page navigation. The router
 * cannot move between the apps, so the navigation is cancelled here and the browser takes over.
 * `target` is an absolute path, or a function that builds it from the requested in-app URL.
 */
export function externalRoute(target: string | ((url: string) => string)): CanActivateFn {
  return (_route, state) => {
    inject(LOCATION_ASSIGN)(typeof target === 'string' ? target : target(state.url));
    return false;
  };
}

/** Route entry that hands a path over to another application (leaf route, never renders). */
export function externalPath(path: string, target: string | ((url: string) => string)): Route {
  return {
    path,
    pathMatch: path.includes('**') ? undefined : 'full',
    canActivate: [externalRoute(target)],
    children: [],
  };
}

/** Target for paths that keep the same address in every application of the origin. */
export const sameUrl = (url: string): string => url;
