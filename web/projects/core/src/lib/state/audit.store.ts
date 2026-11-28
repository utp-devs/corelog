import { Injectable, computed, inject, signal } from '@angular/core';
import { firstValueFrom } from 'rxjs';

import type { ApiError } from '../models';
import type { AuditEntry, AuditFilters, AuditScope } from '../models/audit.model';
import { AuditApiService } from '../services/api/audit-api.service';
import { asApiError } from '../utils/api-error';
import { resetOnLogout } from './session-scoped';

/** Read-only audit log of one scope with cursor paging ("cargar más"). */
@Injectable({ providedIn: 'root' })
export class AuditStore {
  private readonly api = inject(AuditApiService);

  private scope: AuditScope | null = null;
  private generation = 0;
  private readonly entriesState = signal<readonly AuditEntry[]>([]);
  private readonly cursorState = signal<string | null>(null);
  private readonly loadingState = signal(false);
  private readonly loadedState = signal(false);
  private readonly errorState = signal<ApiError | null>(null);
  private readonly filtersState = signal<AuditFilters>({});

  readonly entries = this.entriesState.asReadonly();
  readonly filters = this.filtersState.asReadonly();
  readonly loading = this.loadingState.asReadonly();
  readonly loaded = this.loadedState.asReadonly();
  readonly error = this.errorState.asReadonly();
  readonly hasMore = computed(() => this.cursorState() !== null);
  readonly isEmpty = computed(() => this.loadedState() && this.entriesState().length === 0);

  constructor() {
    resetOnLogout(() => this.reset());
  }

  /** Starts over for a scope and filters. A response of an older request is ignored. */
  async open(scope: AuditScope, filters: AuditFilters = {}): Promise<void> {
    this.scope = scope;
    this.filtersState.set(filters);
    this.entriesState.set([]);
    this.cursorState.set(null);
    this.loadedState.set(false);
    await this.fetch(null);
  }

  async applyFilters(filters: AuditFilters): Promise<void> {
    if (!this.scope) return;
    await this.open(this.scope, filters);
  }

  async loadMore(): Promise<void> {
    const cursor = this.cursorState();
    if (!cursor || this.loadingState()) return;
    await this.fetch(cursor);
  }

  async retry(): Promise<void> {
    await this.fetch(this.entriesState().length > 0 ? this.cursorState() : null);
  }

  /** CSV of the whole log for the current filters. */
  exportCsv(): Promise<Blob> {
    if (!this.scope) return Promise.reject(new Error('no audit scope'));
    return firstValueFrom(this.api.exportCsv(this.scope, this.filtersState()));
  }

  reset(): void {
    this.generation += 1;
    this.scope = null;
    this.entriesState.set([]);
    this.cursorState.set(null);
    this.loadingState.set(false);
    this.loadedState.set(false);
    this.errorState.set(null);
    this.filtersState.set({});
  }

  private async fetch(cursor: string | null): Promise<void> {
    const scope = this.scope;
    if (!scope) return;
    const generation = ++this.generation;
    this.loadingState.set(true);
    this.errorState.set(null);
    try {
      const page = await firstValueFrom(this.api.list(scope, this.filtersState(), { cursor }));
      if (generation !== this.generation) return;
      this.entriesState.update((current) =>
        cursor ? [...current, ...page.entries] : page.entries,
      );
      this.cursorState.set(page.nextCursor);
      this.loadedState.set(true);
    } catch (error) {
      if (generation !== this.generation) return;
      this.errorState.set(asApiError(error));
    } finally {
      if (generation === this.generation) this.loadingState.set(false);
    }
  }
}
