import { inject } from '@angular/core';
import { ResolveFn } from '@angular/router';

import { CurrentProjectStore } from '../state/current-project.store';

/** Name of the route parameter that carries the project key (or UUID) in `/projects/:key/...`. */
export const PROJECT_ROUTE_PARAM = 'key';

/**
 * Starts the read of the `:key` route parameter (project key or UUID) in CurrentProjectStore, which
 * also loads the caller's permissions. It never blocks the navigation: the project area shows its
 * loading state, and a project that does not exist (404) or is not allowed (403) leaves the failure in
 * `CurrentProjectStore.error`, which the area renders as a full-page state. Resolves to the identifier.
 */
export const projectRouteResolver: ResolveFn<string> = (route) => {
  const identifier = route.paramMap.get(PROJECT_ROUTE_PARAM) ?? '';
  inject(CurrentProjectStore).open(identifier);
  return identifier;
};
