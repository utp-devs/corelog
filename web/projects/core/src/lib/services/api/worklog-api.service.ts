import { HttpClient, HttpParams } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable, map } from 'rxjs';

import { API_URL } from '../../config/api.config';
import {
  LogWorkInput,
  RemainingMode,
  Timesheet,
  UpdateWorklogInput,
  Worklog,
  WorklogList,
  WorklogTotals,
} from '../../models/worklog.model';
import { silentRequest } from '../../utils/silent-request';

type Dto = Record<string, unknown>;

interface Envelope<T> {
  data: T;
}

function toWorklog(dto: Dto): Worklog {
  return {
    id: String(dto['id']),
    issueId: String(dto['issue_id']),
    issueKey: String(dto['issue_key'] ?? ''),
    authorId: String(dto['author_id']),
    authorName: String(dto['author_name'] ?? ''),
    startedAt: String(dto['started_at']),
    timeSpentSeconds: Number(dto['time_spent_seconds']),
    timeSpent: String(dto['time_spent'] ?? ''),
    description: String(dto['description'] ?? ''),
    remainingMode: (dto['remaining_mode'] as RemainingMode) ?? 'auto',
    createdAt: String(dto['created_at']),
    updatedAt: String(dto['updated_at']),
  };
}

function toTotals(dto: Dto): WorklogTotals {
  return {
    originalEstimateSeconds: (dto['original_estimate_seconds'] as number | null) ?? null,
    remainingEstimateSeconds: (dto['remaining_estimate_seconds'] as number | null) ?? null,
    timeSpentSeconds: Number(dto['time_spent_seconds'] ?? 0),
    originalEstimate: (dto['original_estimate'] as string | null) ?? null,
    remainingEstimate: (dto['remaining_estimate'] as string | null) ?? null,
    timeSpent: String(dto['time_spent'] ?? '0m'),
  };
}

/** Worklogs of an issue, estimates and the personal timesheet. */
@Injectable({ providedIn: 'root' })
export class WorklogApiService {
  private readonly http = inject(HttpClient);
  private readonly api = inject(API_URL);

  list(issueId: string): Observable<WorklogList> {
    return this.http
      .get<Envelope<Dto>>(`${this.api}/issues/${issueId}/worklogs`, { context: silentRequest() })
      .pipe(map((body) => this.toList(body.data)));
  }

  log(issueId: string, input: LogWorkInput): Observable<WorklogList['totals']> {
    return this.http
      .post<Envelope<Dto>>(
        `${this.api}/issues/${issueId}/worklogs`,
        {
          time_spent: input.timeSpent,
          started_at: input.startedAt,
          description: input.description,
          remaining: input.remaining,
        },
        { context: silentRequest() },
      )
      .pipe(map((body) => toTotals(body.data['totals'] as Dto)));
  }

  update(id: string, input: UpdateWorklogInput): Observable<WorklogTotals> {
    return this.http
      .patch<Envelope<Dto>>(
        `${this.api}/worklogs/${id}`,
        {
          time_spent: input.timeSpent,
          started_at: input.startedAt,
          description: input.description,
        },
        { context: silentRequest() },
      )
      .pipe(map((body) => toTotals(body.data['totals'] as Dto)));
  }

  remove(id: string): Observable<void> {
    return this.http.delete<void>(`${this.api}/worklogs/${id}`, { context: silentRequest() });
  }

  /** Sets (text), clears (null) or leaves (undefined) each estimate. */
  setEstimates(
    issueId: string,
    estimates: { original?: string | null; remaining?: string | null },
  ): Observable<WorklogTotals> {
    return this.http
      .put<Envelope<Dto>>(`${this.api}/issues/${issueId}/estimates`, estimates, {
        context: silentRequest(),
      })
      .pipe(map((body) => toTotals(body.data)));
  }

  timesheet(from: string, to: string): Observable<Timesheet> {
    const params = new HttpParams().set('from', from).set('to', to);
    return this.http
      .get<Envelope<Dto>>(`${this.api}/me/timesheet`, { params, context: silentRequest() })
      .pipe(
        map((body) => ({
          from: String(body.data['from']),
          to: String(body.data['to']),
          totalSeconds: Number(body.data['total_seconds'] ?? 0),
          total: String(body.data['total'] ?? '0m'),
          days: ((body.data['days'] as Dto[] | undefined) ?? []).map((day) => ({
            date: String(day['date']),
            totalSeconds: Number(day['total_seconds'] ?? 0),
            total: String(day['total'] ?? '0m'),
            capacitySeconds: Number(day['capacity_seconds'] ?? 0),
            entries: ((day['entries'] as Dto[] | undefined) ?? []).map((entry) => ({
              id: String(entry['id']),
              issueId: String(entry['issue_id']),
              issueKey: String(entry['issue_key']),
              issueTitle: String(entry['issue_title'] ?? ''),
              startedAt: String(entry['started_at']),
              timeSpentSeconds: Number(entry['time_spent_seconds']),
              timeSpent: String(entry['time_spent'] ?? ''),
              description: String(entry['description'] ?? ''),
            })),
          })),
        })),
      );
  }

  private toList(data: Dto): WorklogList {
    return {
      worklogs: ((data['worklogs'] as Dto[] | undefined) ?? []).map(toWorklog),
      totals: toTotals((data['totals'] as Dto | undefined) ?? {}),
    };
  }
}
