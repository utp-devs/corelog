import { InjectionToken } from '@angular/core';

import { ApiError } from '../models';

/** Returns the id of the active organization, or null when none is selected. */
export type OrganizationIdProvider = () => string | null;

/** Code of the error raised when a request needs an active organization and there is none. */
export const NO_CURRENT_ORGANIZATION_CODE = 'org.no_current';

/**
 * Source of the active organization id used by the API services to add `organization_id` to
 * their queries. The default provider returns null; the application replaces it with the
 * organization store.
 */
export const ORGANIZATION_ID_PROVIDER = new InjectionToken<OrganizationIdProvider>(
  'ORGANIZATION_ID_PROVIDER',
  {
    providedIn: 'root',
    factory: () => (): string | null => null,
  },
);

export function noCurrentOrganizationError(): ApiError {
  return {
    status: 0,
    code: NO_CURRENT_ORGANIZATION_CODE,
    message: 'No hay una organización activa. Elige o crea una organización para continuar.',
    details: [],
  };
}
