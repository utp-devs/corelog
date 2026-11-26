import { AppError } from '@/shared/lib/app-error'
import { getOrgId } from './org-context'

export const NO_CURRENT_ORG_CODE = 'org.no_current'

/**
 * Adds organization_id (the active organization) to the query or body params of a request.
 * Throws AppError with code org.no_current when no organization is active.
 */
export function withOrg(params: Record<string, unknown> = {}): Record<string, unknown> {
  const organizationId = getOrgId()
  if (!organizationId) {
    throw new AppError({
      status: 0,
      code: NO_CURRENT_ORG_CODE,
      message: 'No hay una organización activa. Elige o crea una organización para continuar.',
    })
  }
  return { ...params, organization_id: organizationId }
}
