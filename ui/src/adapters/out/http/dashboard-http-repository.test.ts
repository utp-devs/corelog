import { describe, expect, it } from 'vitest'

import { toMyDashboard } from './dashboard-http-repository'

describe('toMyDashboard', () => {
  it('maps the summary from snake_case and keeps missing dates undefined', () => {
    const summary = toMyDashboard({
      assigned_open: 3,
      assigned_by_status: { open: 1, in_progress: 2 },
      overdue: 1,
      project_count: 2,
      active_sprints: [{ id: 's-1', name: 'Sprint 1', project_key: 'CORE', end_date: null }],
      recent_issues: [{ id: 'i-1', title: 'Revisar', status: 'open', priority: 'high', project_key: 'CORE', due_date: '2026-11-30' }],
    })

    expect(summary.assignedOpen).toBe(3)
    expect(summary.assignedByStatus.in_progress).toBe(2)
    expect(summary.activeSprints[0]).toEqual({ id: 's-1', name: 'Sprint 1', projectKey: 'CORE', endDate: undefined })
    expect(summary.recentIssues[0]).toMatchObject({ projectKey: 'CORE', dueDate: '2026-11-30' })
  })
})
