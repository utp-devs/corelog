import { Outlet } from 'react-router-dom'

import { useAuthStore } from '@/adapters/in/state/auth.store'
import { ErrorPage } from '@/shared/ui/ErrorPage'

/**
 * Nested routes for platform admins. Other users see the 403 page in place of the content
 * (no silent redirect), so the reason for the missing page is explicit.
 */
export function RequirePlatformAdmin() {
  const role = useAuthStore((s) => s.user?.role)

  if (role !== 'admin') {
    return <ErrorPage variant={403} />
  }

  return <Outlet />
}
