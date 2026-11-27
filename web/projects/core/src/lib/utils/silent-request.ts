import { HttpContext } from '@angular/common/http';

import { SKIP_ERROR_NOTIFICATION, SKIP_LOADING } from '../config/http-context';

/** Request whose failures are rendered by the caller instead of a toast. */
export function silentRequest(): HttpContext {
  return new HttpContext().set(SKIP_ERROR_NOTIFICATION, true);
}

/** Background request: no toast and no global loading bar (periodic polling). */
export function backgroundRequest(): HttpContext {
  return new HttpContext().set(SKIP_ERROR_NOTIFICATION, true).set(SKIP_LOADING, true);
}
