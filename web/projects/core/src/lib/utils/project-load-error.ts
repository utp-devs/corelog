import { ApiError } from '../models';

/** Access failures that have a full-page error (403 forbidden, 404 not found). */
export type ProjectAccessVariant = 403 | 404;

/** Returns 403 or 404 when the failure is an access problem; null for anything else (show an inline error). */
export function projectAccessVariantOf(
  error: ApiError | null | undefined,
): ProjectAccessVariant | null {
  const status = error?.status;
  if (status === 403 || status === 404) return status;
  return null;
}
