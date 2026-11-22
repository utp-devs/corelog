import type { DashboardIssue, DashboardSprint, MyDashboard } from '@/core/projects/domain/dashboard'
import { apiClient } from './api-client'

interface MyDashboardDto {
  assigned_open: number
  assigned_by_status: Record<string, number>
  overdue: number
  project_count: number
  active_sprints: { id: string; name: string; project_key: string; end_date?: string | null }[]
  recent_issues: { id: string; title: string; status: string; priority: string; project_key: string; due_date?: string | null }[]
}

export function toMyDashboard(dto: MyDashboardDto): MyDashboard {
  const sprints: DashboardSprint[] = dto.active_sprints.map((sprint) => ({
    id: sprint.id, name: sprint.name, projectKey: sprint.project_key, endDate: sprint.end_date ?? undefined,
  }))
  const issues: DashboardIssue[] = dto.recent_issues.map((issue) => ({
    id: issue.id, title: issue.title, status: issue.status, priority: issue.priority,
    projectKey: issue.project_key, dueDate: issue.due_date ?? undefined,
  }))
  return {
    assignedOpen: dto.assigned_open,
    assignedByStatus: dto.assigned_by_status,
    overdue: dto.overdue,
    projectCount: dto.project_count,
    activeSprints: sprints,
    recentIssues: issues,
  }
}

export class DashboardHttpRepository {
  async me(): Promise<MyDashboard> {
    const { data } = await apiClient.get<{ data: MyDashboardDto }>('/dashboard/me')
    return toMyDashboard(data.data)
  }
}
