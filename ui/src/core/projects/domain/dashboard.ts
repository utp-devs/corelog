export interface DashboardSprint {
  id: string
  name: string
  projectKey: string
  endDate?: string
}

export interface DashboardIssue {
  id: string
  title: string
  status: string
  priority: string
  projectKey: string
  dueDate?: string
}

export interface MyDashboard {
  assignedOpen: number
  assignedByStatus: Record<string, number>
  overdue: number
  projectCount: number
  activeSprints: DashboardSprint[]
  recentIssues: DashboardIssue[]
}
