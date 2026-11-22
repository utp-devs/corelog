/* eslint-disable react-refresh/only-export-components */
import { Spin } from 'antd'
import type { ReactNode } from 'react'
import { lazy, Suspense } from 'react'
import { createBrowserRouter, Navigate } from 'react-router-dom'

import { AdminRoute } from '@/adapters/in/ui/common/AdminRoute'
import { ProtectedRoute } from '@/adapters/in/ui/common/ProtectedRoute'
import { AppLayout } from '@/adapters/in/ui/layout/AppLayout'

const LoginPage = lazy(() => import('@/adapters/in/ui/auth/LoginPage').then(m => ({ default: m.LoginPage })))
const RegisterPage = lazy(() => import('@/adapters/in/ui/auth/RegisterPage').then(m => ({ default: m.RegisterPage })))
const ProfilePage = lazy(() => import('@/adapters/in/ui/profile/ProfilePage').then(m => ({ default: m.ProfilePage })))
const TeamsPage = lazy(() => import('@/adapters/in/ui/teams/TeamsPage').then(m => ({ default: m.TeamsPage })))
const NewTicketPage = lazy(() => import('@/adapters/in/ui/tickets/NewTicketPage').then(m => ({ default: m.NewTicketPage })))
const TicketDetailPage = lazy(() => import('@/adapters/in/ui/tickets/TicketDetailPage').then(m => ({ default: m.TicketDetailPage })))
const TicketsListPage = lazy(() => import('@/adapters/in/ui/tickets/TicketsListPage').then(m => ({ default: m.TicketsListPage })))
const NotFoundPage = lazy(() => import('@/adapters/in/ui/common/NotFoundPage').then(m => ({ default: m.NotFoundPage })))
const ProjectsPage = lazy(() => import('@/adapters/in/ui/projects/ProjectsPage').then(m => ({ default: m.ProjectsPage })))
const ProjectWorkspace = lazy(() => import('@/adapters/in/ui/projects/ProjectWorkspace').then(m => ({ default: m.ProjectWorkspace })))
const ProjectBoardPage = lazy(() => import('@/adapters/in/ui/projects/ProjectBoardPage').then(m => ({ default: m.ProjectBoardPage })))
const ProjectBacklogPage = lazy(() => import('@/adapters/in/ui/projects/ProjectBacklogPage').then(m => ({ default: m.ProjectBacklogPage })))
const ProjectSprintsPage = lazy(() => import('@/adapters/in/ui/projects/ProjectSprintsPage').then(m => ({ default: m.ProjectSprintsPage })))
const MyDashboardPage = lazy(() => import('@/adapters/in/ui/dashboard/MyDashboardPage').then(m => ({ default: m.MyDashboardPage })))
const ProjectPermissionsPage = lazy(() => import('@/adapters/in/ui/projects/ProjectPermissionsPage').then(m => ({ default: m.ProjectPermissionsPage })))
const ProjectCalendarPage = lazy(() => import('@/adapters/in/ui/projects/ProjectCalendarPage').then(m => ({ default: m.ProjectCalendarPage })))

function SuspenseWrapper({ children }: { children: ReactNode }) {
  return (
    <Suspense
      fallback={
        <div className="route-loading">
          <Spin size="large" />
        </div>
      }
    >
      {children}
    </Suspense>
  )
}

export const router = createBrowserRouter([
  { path: '/login', element: <SuspenseWrapper><LoginPage /></SuspenseWrapper> },
  { path: '/register', element: <SuspenseWrapper><RegisterPage /></SuspenseWrapper> },
  {
    element: <ProtectedRoute />,
    children: [
      {
        element: <AppLayout />,
        children: [
          { index: true, element: <Navigate to="/issues" replace /> },
          { path: 'issues', element: <SuspenseWrapper><TicketsListPage /></SuspenseWrapper> },
          { path: 'issues/new', element: <SuspenseWrapper><NewTicketPage /></SuspenseWrapper> },
          { path: 'issues/:id', element: <SuspenseWrapper><TicketDetailPage /></SuspenseWrapper> },
          { path: 'profile', element: <SuspenseWrapper><ProfilePage /></SuspenseWrapper> },
          { path: 'projects', element: <SuspenseWrapper><ProjectsPage /></SuspenseWrapper> },
          {
            path: 'projects/:id',
            element: <SuspenseWrapper><ProjectWorkspace /></SuspenseWrapper>,
            children: [
              { index: true, element: <Navigate to="board" replace /> },
              { path: 'board', element: <SuspenseWrapper><ProjectBoardPage /></SuspenseWrapper> },
              { path: 'backlog', element: <SuspenseWrapper><ProjectBacklogPage /></SuspenseWrapper> },
              { path: 'sprints', element: <SuspenseWrapper><ProjectSprintsPage /></SuspenseWrapper> },
              { path: 'calendar', element: <SuspenseWrapper><ProjectCalendarPage /></SuspenseWrapper> },
              { path: 'permisos', element: <SuspenseWrapper><ProjectPermissionsPage /></SuspenseWrapper> },
            ],
          },
          { path: 'dashboard', element: <SuspenseWrapper><MyDashboardPage /></SuspenseWrapper> },
          {
            element: <AdminRoute />,
            children: [{ path: 'teams', element: <SuspenseWrapper><TeamsPage /></SuspenseWrapper> }],
          },
        ],
      },
    ],
  },
  { path: '*', element: <SuspenseWrapper><NotFoundPage /></SuspenseWrapper> },
])
