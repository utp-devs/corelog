import { CanActivateChildFn, CanActivateFn } from '@angular/router';
import { inject } from '@angular/core';

import { SessionService } from '../services/session.service';

/** Lets signed-in users through; anyone else is sent to the login page (with returnUrl). */
export const authGuard: CanActivateFn & CanActivateChildFn = () => {
  const session = inject(SessionService);
  if (session.isAuthenticated()) {
    return true;
  }
  session.redirectToLogin();
  return false;
};
