import { Outlet } from 'react-router-dom'

import { useCurrentOrg } from '@/adapters/in/state/org.store'
import { canManageOrg } from '@/core/organizations/domain/organization'
import { ErrorPage } from '@/shared/ui/ErrorPage'

/**
 * Nested routes for organization administration (settings, members, invitations). Only owners and
 * admins of the active organization get the content; everyone else sees the 403 page. Put it inside
 * RequireOrg, which loads the organizations and therefore the caller's role.
 */
export function RequireOrgAdmin() {
  const { role } = useCurrentOrg()

  if (!canManageOrg(role)) {
    return <ErrorPage variant={403} />
  }

  return <Outlet />
}
