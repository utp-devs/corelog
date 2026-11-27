import { inject } from '@angular/core';
import { CanActivateFn, Router } from '@angular/router';
import { HOME_URL, LOCATION_ASSIGN, SessionService } from '@corelog/core';

/**
 * Root of the portal. Signed-in users go to the workspace home with a full-page navigation; everyone
 * else is redirected to the login page.
 */
export const landingGuard: CanActivateFn = () => {
  const session = inject(SessionService);
  if (session.isAuthenticated()) {
    inject(LOCATION_ASSIGN)(inject(HOME_URL));
    return false;
  }
  return inject(Router).parseUrl('/login');
};
