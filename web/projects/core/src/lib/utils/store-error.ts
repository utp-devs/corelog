import { HttpErrorResponse } from '@angular/common/http';

import { ApiError } from '../models';
import { asApiError, isApiError } from './api-error';

/**
 * Normalizes a failure stored by a domain store. API errors keep their code; unexpected
 * (non-HTTP) failures get the fallback message so the store always holds a readable error.
 */
export function storeError(error: unknown, fallbackMessage: string): ApiError {
  if (isApiError(error)) return error;
  if (error instanceof HttpErrorResponse) return asApiError(error);
  return { status: 0, code: 'unknown', message: fallbackMessage, details: [] };
}
