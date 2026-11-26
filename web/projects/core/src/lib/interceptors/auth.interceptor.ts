import { HttpInterceptorFn } from '@angular/common/http';
import { inject } from '@angular/core';

import { API_URL } from '../config/api.config';
import { SKIP_AUTH } from '../config/http-context';
import { SessionService } from '../services/session.service';

/** Adds `Authorization: Bearer <token>` to API requests. Public requests are left untouched. */
export const authInterceptor: HttpInterceptorFn = (request, next) => {
  if (request.context.get(SKIP_AUTH) || !request.url.startsWith(inject(API_URL))) {
    return next(request);
  }
  const token = inject(SessionService).token();
  if (!token) {
    return next(request);
  }
  return next(request.clone({ setHeaders: { Authorization: `Bearer ${token}` } }));
};
