import type { OrgRole } from '../models/organization.model';
import type { AppRole } from '../models/user.model';
import { canManageOrg } from '../utils/org-permissions';

/** Capabilities that gate navigation items. The server stays the authority; these only drive visibility. */
export type Permission = 'org.admin' | 'project.permissions' | 'platform.admin';

/**
 * Permissions granted to a user. Platform admins and owners/admins of the active organization get the
 * organization administration; platform admins also get project permissions and the platform pages (legal documents). The organization role
 * never grants project permissions.
 */
export function permissionsFor(
  user: { readonly role: AppRole } | null | undefined,
  orgRole?: OrgRole | null,
): ReadonlySet<Permission> {
  const granted = new Set<Permission>();
  if (canManageOrg(orgRole)) {
    granted.add('org.admin');
  }
  if (user?.role === 'admin') {
    granted.add('org.admin');
    granted.add('project.permissions');
    granted.add('platform.admin');
  }
  return granted;
}

/** True when the item needs no permission, or the granted set contains the required one. */
export function hasPermission(granted: ReadonlySet<Permission>, required?: Permission): boolean {
  return required === undefined || granted.has(required);
}
