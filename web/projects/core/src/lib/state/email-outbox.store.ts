import { Injectable, computed, inject, signal } from '@angular/core';
import { firstValueFrom } from 'rxjs';

import type { ApiError } from '../models/api-error.model';
import type { EmailOutboxEntry, EmailOutboxStatus } from '../models/email-outbox.model';
import { EmailOutboxApiService } from '../services/api/email-outbox-api.service';
import { asApiError } from '../utils/api-error';
import { resetOnLogout } from './session-scoped';

export const EMAIL_OUTBOX_PAGE_SIZE = 25;

/** Outgoing email queue for platform administrators: one page, a status filter and retry of dead rows. */
@Injectable({ providedIn: 'root' })
export class EmailOutboxStore {
  private readonly api = inject(EmailOutboxApiService);
  private generation = 0;

  private readonly itemsState = signal<readonly EmailOutboxEntry[]>([]);
  private readonly statusFilterState = signal<EmailOutboxStatus | null>(null);
  private readonly pageState = signal(1);
  private readonly totalState = signal(0);
  private readonly loadingState = signal(false);
  private readonly loadedState = signal(false);
  private readonly errorState = signal<ApiError | null>(null);
  private readonly retryingState = signal<ReadonlySet<string>>(new Set());

  readonly items = this.itemsState.asReadonly();
  readonly statusFilter = this.statusFilterState.asReadonly();
  readonly page = this.pageState.asReadonly();
  readonly total = this.totalState.asReadonly();
  readonly loading = this.loadingState.asReadonly();
  readonly loaded = this.loadedState.asReadonly();
  readonly error = this.errorState.asReadonly();
  readonly retrying = this.retryingState.asReadonly();
  readonly pageCount = computed(() =>
    Math.max(1, Math.ceil(this.totalState() / EMAIL_OUTBOX_PAGE_SIZE)),
  );
  readonly isEmpty = computed(() => this.loadedState() && this.itemsState().length === 0);

  constructor() {
    resetOnLogout(() => this.reset());
  }

  async setFilter(status: EmailOutboxStatus | null): Promise<void> {
    this.statusFilterState.set(status);
    this.pageState.set(1);
    await this.load();
  }

  async goTo(page: number): Promise<void> {
    this.pageState.set(Math.min(Math.max(1, page), this.pageCount()));
    await this.load();
  }

  /** Reads the current page. `background` keeps the loading bar off (auto refresh) and keeps rows on failure. */
  async load(background = false): Promise<void> {
    const generation = ++this.generation;
    if (!background) this.loadingState.set(true);
    try {
      const page = await firstValueFrom(
        this.api.list(
          {
            status: this.statusFilterState() ?? undefined,
            page: this.pageState(),
            pageSize: EMAIL_OUTBOX_PAGE_SIZE,
          },
          background,
        ),
      );
      if (generation !== this.generation) return;
      this.itemsState.set(page.items);
      this.totalState.set(page.total);
      this.errorState.set(null);
      this.loadedState.set(true);
    } catch (failure) {
      if (generation !== this.generation) return;
      this.errorState.set(asApiError(failure));
    } finally {
      if (generation === this.generation) this.loadingState.set(false);
    }
  }

  /** Puts a dead email back in the queue. Resolves with the failure, or null on success. */
  async retry(id: string): Promise<ApiError | null> {
    this.retryingState.update((ids) => new Set(ids).add(id));
    try {
      await firstValueFrom(this.api.retry(id));
      await this.load(true);
      return null;
    } catch (failure) {
      return asApiError(failure);
    } finally {
      this.retryingState.update((ids) => {
        const next = new Set(ids);
        next.delete(id);
        return next;
      });
    }
  }

  reset(): void {
    this.generation += 1;
    this.itemsState.set([]);
    this.statusFilterState.set(null);
    this.pageState.set(1);
    this.totalState.set(0);
    this.loadingState.set(false);
    this.loadedState.set(false);
    this.errorState.set(null);
    this.retryingState.set(new Set());
  }
}
