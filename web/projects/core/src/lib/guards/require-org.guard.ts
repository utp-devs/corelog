import { inject, InjectionToken } from '@angular/core';
import { CanActivateChildFn, CanActivateFn } from '@angular/router';
import { catchError, map, of } from 'rxjs';

import { LOCATION_ASSIGN } from '../config/navigation';
import { OrganizationStore } from '../state/organization.store';

/** Onboarding page of the portal (outside the workspace app, so a full-page navigation). */
export const ONBOARDING_URL = new InjectionToken<string>('ONBOARDING_URL', {
  providedIn: 'root',
  factory: () => '/onboarding',
});

/**
 * Routes that need an active organization. Loads the caller's organizations once (the route waits
 * for the request), sends users without organizations to the onboarding page and lets the others
 * through. A failed load blocks the route; the shell shows the error and offers a retry through
 * OrganizationStore.fetchOrgs().
 */
export const requireOrgGuard: CanActivateFn & CanActivateChildFn = () => {
  const store = inject(OrganizationStore);
  const navigate = inject(LOCATION_ASSIGN);
  const onboardingUrl = inject(ONBOARDING_URL);

  return store.ensureOrgs().pipe(
    map((orgs) => {
      if (orgs.length > 0) {
        return true;
      }
      navigate(onboardingUrl);
      return false;
    }),
    catchError(() => of(false)),
  );
};
