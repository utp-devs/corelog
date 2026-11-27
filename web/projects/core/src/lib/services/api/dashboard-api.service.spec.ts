import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';

import { provideCorelogTesting } from '../../../testing';
import type { MyDashboard } from '../../models/dashboard.model';
import { DashboardApiService } from './dashboard-api.service';

function setup(): { service: DashboardApiService; backend: HttpTestingController } {
  TestBed.configureTestingModule({
    providers: [provideCorelogTesting(), provideHttpClient(), provideHttpClientTesting()],
  });
  return {
    service: TestBed.inject(DashboardApiService),
    backend: TestBed.inject(HttpTestingController),
  };
}

describe('DashboardApiService', () => {
  it('maps the summary from snake_case and keeps missing dates undefined', () => {
    const { service, backend } = setup();
    let summary: MyDashboard | undefined;

    service.me().subscribe((value) => (summary = value));
    backend.expectOne('/api/v1/dashboard/me').flush({
      data: {
        assigned_open: 3,
        assigned_by_status: { open: 1, in_progress: 2 },
        overdue: 1,
        project_count: 2,
        active_sprints: [{ id: 's-1', name: 'Sprint 1', project_key: 'CORE', end_date: null }],
        recent_issues: [
          {
            id: 'i-1',
            title: 'Revisar',
            status: 'open',
            priority: 'high',
            project_key: 'CORE',
            due_date: '2026-11-30',
          },
        ],
      },
    });

    expect(summary?.assignedOpen).toBe(3);
    expect(summary?.assignedByStatus['in_progress']).toBe(2);
    expect(summary?.activeSprints[0]).toEqual({
      id: 's-1',
      name: 'Sprint 1',
      projectKey: 'CORE',
      endDate: undefined,
    });
    expect(summary?.recentIssues[0]).toMatchObject({ projectKey: 'CORE', dueDate: '2026-11-30' });
    backend.verify();
  });
});
