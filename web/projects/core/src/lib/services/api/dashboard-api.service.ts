import { HttpClient } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable, map } from 'rxjs';

import { API_URL } from '../../config/api.config';
import type { DashboardIssue, DashboardSprint, MyDashboard } from '../../models/dashboard.model';
import { silentRequest } from '../../utils/silent-request';

interface MyDashboardDto {
  readonly assigned_open: number;
  readonly assigned_by_status: Readonly<Record<string, number>>;
  readonly overdue: number;
  readonly project_count: number;
  readonly active_sprints: readonly {
    readonly id: string;
    readonly name: string;
    readonly project_key: string;
    readonly end_date?: string | null;
  }[];
  readonly recent_issues: readonly {
    readonly id: string;
    readonly title: string;
    readonly status: string;
    readonly priority: string;
    readonly project_key: string;
    readonly due_date?: string | null;
  }[];
}

interface DataEnvelope<T> {
  readonly data: T;
}

function toMyDashboard(dto: MyDashboardDto): MyDashboard {
  const activeSprints: DashboardSprint[] = dto.active_sprints.map((sprint) => ({
    id: sprint.id,
    name: sprint.name,
    projectKey: sprint.project_key,
    endDate: sprint.end_date ?? undefined,
  }));
  const recentIssues: DashboardIssue[] = dto.recent_issues.map((issue) => ({
    id: issue.id,
    title: issue.title,
    status: issue.status,
    priority: issue.priority,
    projectKey: issue.project_key,
    dueDate: issue.due_date ?? undefined,
  }));
  return {
    assignedOpen: dto.assigned_open,
    assignedByStatus: dto.assigned_by_status,
    overdue: dto.overdue,
    projectCount: dto.project_count,
    activeSprints,
    recentIssues,
  };
}

/** Personal summary of the signed-in user. */
@Injectable({ providedIn: 'root' })
export class DashboardApiService {
  private readonly http = inject(HttpClient);
  private readonly baseUrl = inject(API_URL);

  me(): Observable<MyDashboard> {
    return this.http
      .get<DataEnvelope<MyDashboardDto>>(`${this.baseUrl}/dashboard/me`, {
        context: silentRequest(),
      })
      .pipe(map((response) => toMyDashboard(response.data)));
  }
}
