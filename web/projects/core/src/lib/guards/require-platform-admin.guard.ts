import { inject } from '@angular/core';
import { CanActivateChildFn, CanActivateFn, Router, UrlTree } from '@angular/router';

import { AuthStore } from '../state/auth.store';
import { FORBIDDEN_ROUTE } from './require-org-admin.guard';

/** Platform pages (legal documents): only platform admins get through; everyone else sees the 403 page. */
export const requirePlatformAdminGuard: CanActivateFn & CanActivateChildFn = (): true | UrlTree => {
  const auth = inject(AuthStore);
  return auth.isPlatformAdmin() ? true : inject(Router).parseUrl(inject(FORBIDDEN_ROUTE));
};
