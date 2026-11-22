import { Navigate, Outlet } from 'react-router-dom'

import { useAuthStore } from '@/adapters/in/state/auth.store'

export function AdminRoute() {
  const role = useAuthStore((s) => s.user?.role)

  if (role !== 'admin') {
    return <Navigate to="/issues" replace />
  }

  return <Outlet />
}
