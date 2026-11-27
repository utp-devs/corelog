import { noCurrentOrganizationError } from './org-id-provider';

type OrgIdProvider = () => string | null;

let provider: OrgIdProvider = () => null;

/**
 * Registers the function that returns the active organization id. The OrganizationStore registers
 * itself when it is created, so shared code reads the id without depending on the store.
 * Last registration wins.
 */
export function setOrgIdProvider(fn: OrgIdProvider): void {
  provider = fn;
}

/** Active organization id, or null when none is selected (or no provider is registered yet). */
export function getOrgId(): string | null {
  return provider();
}

/**
 * Adds organization_id (the active organization) to the query or body params of a request.
 * Throws an ApiError with code org.no_current when no organization is active.
 */
export function withOrg(params: Readonly<Record<string, unknown>> = {}): Record<string, unknown> {
  const organizationId = getOrgId();
  if (!organizationId) {
    throw noCurrentOrganizationError();
  }
  return { ...params, organization_id: organizationId };
}
