import { Spin } from 'antd'
import { Navigate, Outlet } from 'react-router-dom'

import { useAuthStore } from '@/adapters/in/state/auth.store'

import styles from './ProtectedRoute.module.css'

export function ProtectedRoute() {
  const status = useAuthStore((s) => s.status)
  const sessionChecked = useAuthStore((s) => s.sessionChecked)

  if (!sessionChecked) {
    return (
      <div className={styles.loading} role="status" aria-live="polite">
        <Spin size="large" description="Comprobando tu sesión..." />
      </div>
    )
  }

  if (status !== 'authenticated') {
    return <Navigate to="/login" replace />
  }

  return <Outlet />
}
