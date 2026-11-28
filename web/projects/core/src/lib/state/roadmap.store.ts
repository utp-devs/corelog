import { Injectable, computed, inject, signal } from '@angular/core';
import { Observable, tap } from 'rxjs';

import type { Roadmap, RoadmapBar, RoadmapFilter, RoadmapGroupBy } from '../models/roadmap.model';
import { RoadmapApiService } from '../services/api/roadmap-api.service';
import { AsyncState } from './async-state';
import { resetOnLogout } from './session-scoped';

const MESSAGE_FAILED = 'No se pudo cargar la hoja de ruta. Intenta de nuevo.';

/** Roadmap of the project in view, with the grouping, filter and search the person chose. */
@Injectable({ providedIn: 'root' })
export class RoadmapStore {
  private readonly api = inject(RoadmapApiService);

  readonly roadmap = new AsyncState<Roadmap>();

  readonly groupBy = signal<RoadmapGroupBy>('epic');
  readonly filter = signal<RoadmapFilter>('all');
  readonly query = signal('');

  private projectId: string | null = null;

  /** Bars of the current grouping. */
  readonly bars = computed<readonly RoadmapBar[]>(() => this.roadmap.value()?.bars ?? []);
  readonly dated = computed(() => this.bars().filter((bar) => bar.dated));
  readonly undated = computed(() => this.bars().filter((bar) => !bar.dated));
  readonly versions = computed<readonly RoadmapBar[]>(() => this.roadmap.value()?.versions ?? []);
  readonly sprints = computed<readonly RoadmapBar[]>(() => this.roadmap.value()?.sprints ?? []);
  readonly errorMessage = computed(() => (this.roadmap.error() ? MESSAGE_FAILED : null));
  readonly forbidden = computed(() => {
    const status = this.roadmap.error()?.status;
    return status === 403 || status === 404;
  });

  constructor() {
    resetOnLogout(() => this.reset());
  }

  /** Loads the roadmap of a project (its UUID) with the current grouping, filter and search. */
  load(projectId: string): void {
    this.projectId = projectId;
    this.roadmap.load(
      this.api.get(projectId, {
        groupBy: this.groupBy(),
        filter: this.filter(),
        q: this.query().trim() || undefined,
      }),
    );
  }

  reload(): void {
    if (this.projectId) this.load(this.projectId);
  }

  setGroupBy(groupBy: RoadmapGroupBy): void {
    this.groupBy.set(groupBy);
    this.reload();
  }

  setFilter(filter: RoadmapFilter): void {
    this.filter.set(filter);
    this.reload();
  }

  setQuery(query: string): void {
    this.query.set(query);
    this.reload();
  }

  /**
   * Saves a manual order of the epics. The list shows the new order at once and goes back to the previous
   * one when the request fails (the caller handles the error).
   */
  rank(order: readonly string[]): Observable<void> {
    const projectId = this.projectId;
    if (!projectId) throw new Error('The roadmap is not loaded');
    const previous = this.roadmap.value();
    if (previous) {
      const byId = new Map(previous.bars.map((bar) => [bar.id, bar]));
      const bars = order.map((id) => byId.get(id)).filter((bar): bar is RoadmapBar => !!bar);
      const rest = previous.bars.filter((bar) => !order.includes(bar.id));
      this.roadmap.set({ ...previous, bars: [...bars, ...rest] });
    }
    return this.api.rank(projectId, order).pipe(
      tap({
        error: () => {
          if (previous) this.roadmap.set(previous);
        },
      }),
    );
  }

  reset(): void {
    this.roadmap.reset();
    this.projectId = null;
    this.groupBy.set('epic');
    this.filter.set('all');
    this.query.set('');
  }
}
