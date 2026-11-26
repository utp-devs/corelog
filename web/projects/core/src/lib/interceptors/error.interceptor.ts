import { HttpInterceptorFn } from '@angular/common/http';
import { inject } from '@angular/core';
import { catchError, throwError } from 'rxjs';

import { SKIP_AUTH, SKIP_ERROR_NOTIFICATION } from '../config/http-context';
import { SessionService } from '../services/session.service';
import { NotificationService } from '../services/notification.service';
import { asApiError } from '../utils/api-error';

/**
 * Normalizes every failed request into an ApiError. A 401 on a signed-in session ends the session
 * (back to the login page); other errors show a toast unless the request opts out.
 */
export const errorInterceptor: HttpInterceptorFn = (request, next) => {
  const session = inject(SessionService);
  const notifications = inject(NotificationService);

  return next(request).pipe(
    catchError((error: unknown) => {
      const apiError = asApiError(error);
      const sessionRejected =
        apiError.status === 401 && !request.context.get(SKIP_AUTH) && session.isAuthenticated();
      if (sessionRejected) {
        session.expire();
      } else if (!request.context.get(SKIP_ERROR_NOTIFICATION)) {
        notifications.error(apiError.message);
      }
      return throwError(() => apiError);
    }),
  );
};
