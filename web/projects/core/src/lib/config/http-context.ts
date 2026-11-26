import { HttpContext, HttpContextToken } from '@angular/common/http';

/** Request is sent without the bearer token (login, registration, public pages). */
export const SKIP_AUTH = new HttpContextToken<boolean>(() => false);

/** Error is not shown as a toast (the caller renders it). */
export const SKIP_ERROR_NOTIFICATION = new HttpContextToken<boolean>(() => false);

/** Request does not move the global loading bar. */
export const SKIP_LOADING = new HttpContextToken<boolean>(() => false);

/** Context for public requests: no token, no error toast. */
export function publicRequest(): HttpContext {
  return new HttpContext().set(SKIP_AUTH, true).set(SKIP_ERROR_NOTIFICATION, true);
}
