import { asApiError } from '@corelog/core';

/**
 * Message of a failed request. Failures that carry no server message (network or unexpected errors)
 * show the fallback text instead of a generic technical message.
 */
export function messageOr(error: unknown, fallback: string): string {
  const apiError = asApiError(error);
  return apiError.status === 0 && apiError.code === 'unknown' ? fallback : apiError.message;
}
