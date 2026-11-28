import { HttpClient } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable, map } from 'rxjs';

import { API_URL } from '../../config/api.config';
import type { ScheduledJob, ScheduledJobRunStatus } from '../../models/scheduled-job.model';
import { backgroundRequest, silentRequest } from '../../utils/silent-request';

interface JobDto {
  readonly name: string;
  readonly schedule: string;
  readonly registered_at: string;
  readonly last_seen_at: string;
  readonly last_success_at?: string | null;
  readonly last_run?: {
    readonly status: ScheduledJobRunStatus;
    readonly started_at: string;
    readonly finished_at?: string | null;
    readonly duration_ms?: number | null;
    readonly error?: string;
  } | null;
}

function toJob(dto: JobDto): ScheduledJob {
  return {
    name: dto.name,
    schedule: dto.schedule,
    registeredAt: dto.registered_at,
    lastSeenAt: dto.last_seen_at,
    lastSuccessAt: dto.last_success_at ?? null,
    lastRun: dto.last_run
      ? {
          status: dto.last_run.status,
          startedAt: dto.last_run.started_at,
          finishedAt: dto.last_run.finished_at ?? null,
          durationMs: dto.last_run.duration_ms ?? null,
          error: dto.last_run.error ?? '',
        }
      : null,
  };
}

/** Scheduled background jobs (platform administrators). */
@Injectable({ providedIn: 'root' })
export class ScheduledJobApiService {
  private readonly http = inject(HttpClient);
  private readonly baseUrl = inject(API_URL);

  list(background = false): Observable<ScheduledJob[]> {
    return this.http
      .get<{ data: readonly JobDto[] }>(`${this.baseUrl}/admin/jobs`, {
        context: background ? backgroundRequest() : silentRequest(),
      })
      .pipe(map((response) => response.data.map(toJob)));
  }
}
