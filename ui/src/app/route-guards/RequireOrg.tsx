import { useEffect } from 'react'
import { Navigate, Outlet } from 'react-router-dom'
import { Spin } from 'antd'

import { useOrgStore } from '@/adapters/in/state/org.store'
import { ErrorState } from '@/shared/ui/ErrorState'

/**
 * Nested routes that need an active organization. Fetches the caller's organizations once,
 * shows a full-page spinner while loading, sends users without organizations to /onboarding
 * and renders the nested routes otherwise. Put it inside ProtectedRoute (needs a session).
 */
export function RequireOrg() {
  const loaded = useOrgStore((s) => s.loaded)
  const loading = useOrgStore((s) => s.loading)
  const error = useOrgStore((s) => s.error)
  const hasOrgs = useOrgStore((s) => s.orgs.length > 0)
  const fetchOrgs = useOrgStore((s) => s.fetchOrgs)

  useEffect(() => {
    if (!loaded && !loading && !error) {
      void fetchOrgs()
    }
  }, [loaded, loading, error, fetchOrgs])

  if (!loaded) {
    if (error) {
      return (
        <div className="route-loading">
          <ErrorState
            message={error.message}
            onRetry={() => {
              void fetchOrgs()
            }}
          />
        </div>
      )
    }
    return (
      <div className="route-loading">
        <Spin size="large" />
      </div>
    )
  }

  if (!hasOrgs) {
    return <Navigate to="/onboarding" replace />
  }

  return <Outlet />
}
