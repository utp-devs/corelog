import { httpStatusOf } from '@/core/shared/app-error'

/** Access failures that have a full-page ErrorPage (403 forbidden, 404 not found). */
export type AccessVariant = 403 | 404

/** Returns 403 or 404 when the failure is an access problem; null for anything else (show an inline error). */
export function accessVariantOf(error: unknown): AccessVariant | null {
  const status = httpStatusOf(error)
  if (status === 403 || status === 404) return status
  return null
}
