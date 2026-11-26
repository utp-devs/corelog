/* eslint-disable react-refresh/only-export-components */
import { Spin } from 'antd'
import type { ReactNode } from 'react'
import { lazy, Suspense } from 'react'
import { createBrowserRouter, Navigate } from 'react-router-dom'

import { ProtectedRoute } from '@/adapters/in/ui/common/ProtectedRoute'
import { OrgShellLayout } from '@/adapters/in/ui/organizations/OrgShellLayout'
import { RequireOrg } from '@/app/route-guards/RequireOrg'
import { RequireOrgAdmin } from '@/app/route-guards/RequireOrgAdmin'

const InviteAcceptPage = lazy(() => import('@/adapters/in/ui/auth/InviteAcceptPage').then(m => ({ default: m.InviteAcceptPage })))
const OnboardingPage = lazy(() => import('@/adapters/in/ui/auth/OnboardingPage').then(m => ({ default: m.OnboardingPage })))
const LoginPage = lazy(() => import('@/adapters/in/ui/auth/LoginPage').then(m => ({ default: m.LoginPage })))
const RegisterPage = lazy(() => import('@/adapters/in/ui/auth/RegisterPage').then(m => ({ default: m.RegisterPage })))
const ProfilePage = lazy(() => import('@/adapters/in/ui/profile/ProfilePage').then(m => ({ default: m.ProfilePage })))
const PeoplePage = lazy(() => import('@/adapters/in/ui/people/PeoplePage').then(m => ({ default: m.PeoplePage })))
const PersonPage = lazy(() => import('@/adapters/in/ui/people/PersonPage').then(m => ({ default: m.PersonPage })))
const OrgSettingsPage = lazy(() => import('@/adapters/in/ui/organizations/settings').then(m => ({ default: m.OrgSettingsPage })))
const OrgMembersPage = lazy(() => import('@/adapters/in/ui/organizations/members').then(m => ({ default: m.OrgMembersPage })))
const OrgInvitationsPage = lazy(() => import('@/adapters/in/ui/organizations/invitations').then(m => ({ default: m.OrgInvitationsPage })))
const TeamsPage = lazy(() => import('@/adapters/in/ui/teams/TeamsPage').then(m => ({ default: m.TeamsPage })))
const TeamPage = lazy(() => import('@/adapters/in/ui/teams').then(m => ({ default: m.TeamPage })))
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
  // Public: the page previews the invitation and asks for login or register when there is no session.
  { path: '/invite/:token', element: <SuspenseWrapper><InviteAcceptPage /></SuspenseWrapper> },
  {
    element: <ProtectedRoute />,
    children: [
      // Authenticated, outside the app shell: the wizard creates or joins an organization.
      { path: '/onboarding', element: <SuspenseWrapper><OnboardingPage /></SuspenseWrapper> },
      {
        element: <RequireOrg />,
        children: [
          {
            element: <OrgShellLayout />,
            children: [
              { index: true, element: <Navigate to="/issues" replace /> },
              { path: 'issues', element: <SuspenseWrapper><TicketsListPage /></SuspenseWrapper> },
              { path: 'issues/new', element: <SuspenseWrapper><NewTicketPage /></SuspenseWrapper> },
              { path: 'issues/:id', element: <SuspenseWrapper><TicketDetailPage /></SuspenseWrapper> },
              { path: 'profile', element: <SuspenseWrapper><ProfilePage /></SuspenseWrapper> },
              { path: 'profile/:section', element: <SuspenseWrapper><ProfilePage /></SuspenseWrapper> },
              { path: 'people', element: <SuspenseWrapper><PeoplePage /></SuspenseWrapper> },
              { path: 'people/:id', element: <SuspenseWrapper><PersonPage /></SuspenseWrapper> },
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
              { path: 'teams', element: <SuspenseWrapper><TeamsPage /></SuspenseWrapper> },
              { path: 'teams/:id', element: <SuspenseWrapper><TeamPage /></SuspenseWrapper> },
              {
                element: <RequireOrgAdmin />,
                children: [
                  { path: 'organization/settings', element: <SuspenseWrapper><OrgSettingsPage /></SuspenseWrapper> },
                  { path: 'organization/members', element: <SuspenseWrapper><OrgMembersPage /></SuspenseWrapper> },
                  { path: 'organization/invitations', element: <SuspenseWrapper><OrgInvitationsPage /></SuspenseWrapper> },
                ],
              },
            ],
          },
        ],
      },
    ],
  },
  { path: '*', element: <SuspenseWrapper><NotFoundPage /></SuspenseWrapper> },
])
