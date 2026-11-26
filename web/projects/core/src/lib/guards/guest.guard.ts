import { inject } from '@angular/core';
import { CanActivateFn } from '@angular/router';

import { HOME_URL, LOCATION_ASSIGN } from '../config/navigation';
import { SessionService } from '../services/session.service';

/** Only for signed-out users (login page). Signed-in users go to the workspace home. */
export const guestGuard: CanActivateFn = () => {
  const session = inject(SessionService);
  if (!session.isAuthenticated()) {
    return true;
  }
  inject(LOCATION_ASSIGN)(inject(HOME_URL));
  return false;
};
