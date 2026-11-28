import { Injectable, computed, inject, signal } from '@angular/core';
import { Observable, map, switchMap, tap } from 'rxjs';

import type {
  Dashboard,
  DashboardCreateInput,
  DashboardGadget,
  DashboardLayout,
  DashboardListQuery,
  DashboardPage,
  DashboardPatch,
  DashboardShareInput,
} from '../models/dashboard-v2.model';
import { DashboardV2ApiService } from '../services/api/dashboard-v2-api.service';
import { FavoritesApiService } from '../services/api/favorites-api.service';
import { toGadgetInputs } from '../utils/gadget-layout';
import { AsyncState } from './async-state';
import { resetOnLogout } from './session-scoped';

const MESSAGE_LIST_FAILED = 'No se pudieron cargar los paneles. Intenta de nuevo.';
const MESSAGE_LOAD_FAILED = 'No se pudo cargar el panel. Intenta de nuevo.';

/** Dashboards of the signed-in user: the list for the manager and the dashboard in view. */
@Injectable({ providedIn: 'root' })
export class DashboardV2Store {
  private readonly api = inject(DashboardV2ApiService);
  private readonly favoritesApi = inject(FavoritesApiService);

  readonly page = new AsyncState<DashboardPage>();
  readonly current = new AsyncState<Dashboard>();

  private readonly savingState = signal(false);
  readonly saving = this.savingState.asReadonly();

  readonly items = computed(() => this.page.value()?.items ?? []);
  readonly listError = computed(() => (this.page.error() ? MESSAGE_LIST_FAILED : null));
  readonly loadError = computed(() => (this.current.error() ? MESSAGE_LOAD_FAILED : null));
  /** The dashboard is not visible to the viewer (or does not exist). */
  readonly notFound = computed(() => this.current.error()?.status === 404);

  constructor() {
    resetOnLogout(() => this.reset());
  }

  loadList(query: DashboardListQuery = {}): void {
    this.page.load(this.api.list({ pageSize: 100, ...query }));
  }

  /** Loads the personal default dashboard (created with standard gadgets on first use). */
  loadMine(): void {
    this.current.load(this.api.mine());
  }

  loadOne(id: string): void {
    this.current.load(this.api.get(id));
  }

  create(input: DashboardCreateInput): Observable<Dashboard> {
    return this.api.create(input);
  }

  /** Saves name, description, layout or default flag and keeps the dashboard in view in sync. */
  update(id: string, patch: DashboardPatch): Observable<Dashboard> {
    return this.api.update(id, patch).pipe(tap((saved) => this.keepCurrent(saved)));
  }

  remove(id: string): Observable<void> {
    return this.api.remove(id).pipe(tap(() => this.dropFromList(id)));
  }

  duplicate(id: string, name?: string): Observable<Dashboard> {
    return this.api.duplicate(id, name);
  }

  /** Replaces layout and gadgets in one request and shows the stored result. */
  saveGadgets(
    id: string,
    layout: DashboardLayout,
    gadgets: readonly DashboardGadget[],
  ): Observable<Dashboard> {
    this.savingState.set(true);
    return this.api.replaceGadgets(id, layout, toGadgetInputs(gadgets)).pipe(
      tap({
        next: (saved) => {
          this.savingState.set(false);
          this.keepCurrent(saved);
        },
        error: () => this.savingState.set(false),
      }),
    );
  }

  setShares(id: string, shares: readonly DashboardShareInput[]): Observable<Dashboard> {
    return this.api.setShares(id, shares).pipe(tap((saved) => this.keepCurrent(saved)));
  }

  /** Stars or unstars a dashboard; the list and the dashboard in view follow. */
  toggleFavorite(dashboard: Dashboard): Observable<boolean> {
    const next = !dashboard.isFavorite;
    const call = next
      ? this.favoritesApi.star('dashboard', dashboard.id)
      : this.favoritesApi.unstar('dashboard', dashboard.id);
    return call.pipe(
      tap(() => this.markFavorite(dashboard.id, next)),
      map(() => next),
    );
  }

  /** Makes a dashboard the default one and refreshes the list, since the previous default loses the flag. */
  makeDefault(id: string): Observable<Dashboard> {
    return this.update(id, { isDefault: true }).pipe(
      switchMap((saved) =>
        this.api.list({ pageSize: 100 }).pipe(
          tap((page) => this.page.set(page)),
          map(() => saved),
        ),
      ),
    );
  }

  reset(): void {
    this.page.reset();
    this.current.reset();
    this.savingState.set(false);
  }

  private keepCurrent(saved: Dashboard): void {
    const current = this.current.value();
    if (current?.id === saved.id) {
      this.current.set({ ...saved, shares: saved.shares ?? current.shares });
    }
    this.page.update((page) =>
      page
        ? { ...page, items: page.items.map((item) => (item.id === saved.id ? saved : item)) }
        : page,
    );
  }

  private dropFromList(id: string): void {
    this.page.update((page) =>
      page
        ? { ...page, items: page.items.filter((item) => item.id !== id), total: page.total - 1 }
        : page,
    );
  }

  private markFavorite(id: string, isFavorite: boolean): void {
    const current = this.current.value();
    if (current?.id === id) this.current.set({ ...current, isFavorite });
    this.page.update((page) =>
      page
        ? {
            ...page,
            items: page.items.map((item) => (item.id === id ? { ...item, isFavorite } : item)),
          }
        : page,
    );
  }
}
