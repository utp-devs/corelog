export type ScheduledJobRunStatus = 'running' | 'ok' | 'error' | 'abandoned';

export interface ScheduledJobRun {
  readonly status: ScheduledJobRunStatus;
  readonly startedAt: string;
  readonly finishedAt: string | null;
  readonly durationMs: number | null;
  readonly error: string;
}

/** Scheduled background job and its last run. */
export interface ScheduledJob {
  readonly name: string;
  readonly schedule: string;
  readonly registeredAt: string;
  readonly lastSeenAt: string;
  readonly lastSuccessAt: string | null;
  readonly lastRun: ScheduledJobRun | null;
}
