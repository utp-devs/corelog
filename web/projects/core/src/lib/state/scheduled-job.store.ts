import { Injectable, computed, inject, signal } from '@angular/core';
import { firstValueFrom } from 'rxjs';

import type { ApiError } from '../models/api-error.model';
import type { ScheduledJob } from '../models/scheduled-job.model';
import { ScheduledJobApiService } from '../services/api/scheduled-job-api.service';
import { asApiError } from '../utils/api-error';
import { resetOnLogout } from './session-scoped';

/** Scheduled background jobs for platform administrators. */
@Injectable({ providedIn: 'root' })
export class ScheduledJobStore {
  private readonly api = inject(ScheduledJobApiService);
  private generation = 0;

  private readonly jobsState = signal<readonly ScheduledJob[]>([]);
  private readonly loadingState = signal(false);
  private readonly loadedState = signal(false);
  private readonly errorState = signal<ApiError | null>(null);

  readonly jobs = this.jobsState.asReadonly();
  readonly loading = this.loadingState.asReadonly();
  readonly loaded = this.loadedState.asReadonly();
  readonly error = this.errorState.asReadonly();
  readonly isEmpty = computed(() => this.loadedState() && this.jobsState().length === 0);

  constructor() {
    resetOnLogout(() => this.reset());
  }

  /** `background` keeps the loading bar off (auto refresh) and keeps the rows when the call fails. */
  async load(background = false): Promise<void> {
    const generation = ++this.generation;
    if (!background) this.loadingState.set(true);
    try {
      const jobs = await firstValueFrom(this.api.list(background));
      if (generation !== this.generation) return;
      this.jobsState.set(jobs);
      this.errorState.set(null);
      this.loadedState.set(true);
    } catch (failure) {
      if (generation !== this.generation) return;
      this.errorState.set(asApiError(failure));
    } finally {
      if (generation === this.generation) this.loadingState.set(false);
    }
  }

  reset(): void {
    this.generation += 1;
    this.jobsState.set([]);
    this.loadingState.set(false);
    this.loadedState.set(false);
    this.errorState.set(null);
  }
}
