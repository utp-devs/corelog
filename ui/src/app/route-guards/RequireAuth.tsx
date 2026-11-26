import { Navigate, Outlet } from 'react-router-dom'

import { useAuthStore } from '@/adapters/in/state/auth.store'

/** Renders nested routes for an authenticated session; waits for the session check; otherwise goes to /login. */
export function RequireAuth() {
  const status = useAuthStore((s) => s.status)
  const sessionChecked = useAuthStore((s) => s.sessionChecked)

  if (!sessionChecked) {
    return null
  }

  if (status !== 'authenticated') {
    return <Navigate to="/login" replace />
  }

  return <Outlet />
}
