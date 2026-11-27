import { asApiError } from '@corelog/core';

/**
 * Message to show for a failed request. Server and transport failures carry their own message.
 * A generic failure (no response and no known cause, such as a plain Error) shows the fallback text.
 */
export function errorMessage(error: unknown, fallback: string): string {
  const normalized = asApiError(error);
  return normalized.code === 'unknown' ? fallback : normalized.message || fallback;
}
