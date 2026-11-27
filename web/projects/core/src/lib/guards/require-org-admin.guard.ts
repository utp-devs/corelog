import { inject, InjectionToken } from '@angular/core';
import { CanActivateChildFn, CanActivateFn, Router, UrlTree } from '@angular/router';
import { catchError, map, of } from 'rxjs';

import { canManageOrg } from '../utils/org-permissions';
import { OrganizationStore } from '../state/organization.store';

/** Route of the 403 page inside the application that uses the guard. */
export const FORBIDDEN_ROUTE = new InjectionToken<string>('FORBIDDEN_ROUTE', {
  providedIn: 'root',
  factory: () => '/403',
});

/**
 * Routes for organization administration (settings, members, invitations). Only owners and admins of
 * the active organization get through; everyone else is sent to the 403 page. Chain it after
 * requireOrgGuard, which handles the missing-organization case.
 */
export const requireOrgAdminGuard: CanActivateFn & CanActivateChildFn = () => {
  const store = inject(OrganizationStore);
  const router = inject(Router);
  const forbiddenRoute = inject(FORBIDDEN_ROUTE);

  return store.ensureOrgs().pipe(
    map((): true | UrlTree =>
      canManageOrg(store.currentRole()) ? true : router.parseUrl(forbiddenRoute),
    ),
    catchError(() => of(false)),
  );
};
