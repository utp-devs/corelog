import { Injectable, computed, inject, signal } from '@angular/core';
import { Observable, firstValueFrom } from 'rxjs';

import type {
  FilterScope,
  FilterShare,
  FilterShareInput,
  FilterSubscription,
  SaveQueryPayload,
  SavedQuery,
} from '../models/filter.model';
import { FilterApiService } from '../services/api/filter-api.service';
import { asApiError } from '../utils/api-error';
import { AsyncState } from './async-state';
import { resetOnLogout } from './session-scoped';

/** Saved filters of the signed-in user: own, shared with them and favorites. */
@Injectable({ providedIn: 'root' })
export class FilterStore {
  private readonly api = inject(FilterApiService);
  private readonly scopeState = signal<FilterScope>('all');

  readonly list = new AsyncState<SavedQuery[]>();
  readonly scope = this.scopeState.asReadonly();
  readonly filters = computed<readonly SavedQuery[]>(() => this.list.value() ?? []);
  readonly mine = computed(() => this.filters().filter((filter) => filter.access === 'owner'));
  readonly shared = computed(() => this.filters().filter((filter) => filter.access !== 'owner'));
  readonly favorites = computed(() => this.filters().filter((filter) => filter.isFavorite));

  constructor() {
    resetOnLogout(() => this.reset());
  }

  load(scope: FilterScope = 'all'): void {
    this.scopeState.set(scope);
    this.list.load(this.api.list(scope));
  }

  find(id: string | null | undefined): SavedQuery | undefined {
    return id ? this.filters().find((filter) => filter.id === id) : undefined;
  }

  /** Saves a query as a new filter. Rejects with the ApiError (for example `filter.name_taken`). */
  async save(payload: SaveQueryPayload): Promise<SavedQuery> {
    const created = await this.call(this.api.create(payload));
    this.replace(created);
    return created;
  }

  async update(
    id: string,
    changes: Partial<Omit<SaveQueryPayload, 'organizationId'>>,
  ): Promise<SavedQuery> {
    const updated = await this.call(this.api.update(id, changes));
    this.replace(updated);
    return updated;
  }

  async remove(id: string): Promise<void> {
    await this.call(this.api.delete(id));
    this.list.update((current) => (current ?? []).filter((filter) => filter.id !== id));
  }

  async share(id: string, shares: readonly FilterShareInput[]): Promise<readonly FilterShare[]> {
    const saved = await this.call(this.api.setShares(id, shares));
    this.patch(id, { shares: saved });
    return saved;
  }

  async subscribe(id: string, schedule: FilterSubscription): Promise<void> {
    const saved = await this.call(this.api.subscribe(id, schedule));
    this.patch(id, { isSubscribed: true, subscription: saved });
  }

  async unsubscribe(id: string): Promise<void> {
    await this.call(this.api.unsubscribe(id));
    this.patch(id, { isSubscribed: false, subscription: null });
  }

  reset(): void {
    this.list.reset();
    this.scopeState.set('all');
  }

  /** Runs a request and rejects with the normalized ApiError, as the interceptor does for the pages. */
  private async call<T>(source: Observable<T>): Promise<T> {
    try {
      return await firstValueFrom(source);
    } catch (error) {
      throw asApiError(error);
    }
  }

  private patch(id: string, changes: Partial<SavedQuery>): void {
    this.list.update((current) =>
      (current ?? []).map((filter) => (filter.id === id ? { ...filter, ...changes } : filter)),
    );
  }

  /** Puts a filter in the list (new or updated), keeping the list sorted by name. */
  private replace(filter: SavedQuery): void {
    this.list.update((current) =>
      [...(current ?? []).filter((item) => item.id !== filter.id), filter].sort((a, b) =>
        a.name.localeCompare(b.name),
      ),
    );
  }
}
