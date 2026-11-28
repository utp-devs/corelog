import { Injectable, computed, inject, signal } from '@angular/core';
import { firstValueFrom } from 'rxjs';

import type {
  CalendarEventDetail,
  CalendarEventInput,
  CalendarFeedSecret,
  CalendarFeedState,
  CalendarItem,
  CalendarItemKind,
  CalendarViewMode,
  CalendarWindow,
  RsvpResponse,
} from '../models/calendar-v2.model';
import { CalendarV2ApiService } from '../services/api/calendar-v2-api.service';
import { groupItemsByDay, shiftAnchor, startOfDay, viewRange } from '../utils/calendar-grid';
import { AsyncState } from './async-state';
import { resetOnLogout } from './session-scoped';

/** Filter groups of the legend; each one covers one or more item kinds. */
export type CalendarKindGroup = 'events' | 'issues' | 'sprints' | 'versions';

export const KIND_GROUPS: Readonly<Record<CalendarKindGroup, readonly CalendarItemKind[]>> = {
  events: ['event'],
  issues: ['issue_due', 'issue_start'],
  sprints: ['sprint_start', 'sprint_end'],
  versions: ['version_release'],
};

export const ALL_KIND_GROUPS: readonly CalendarKindGroup[] = [
  'events',
  'issues',
  'sprints',
  'versions',
];

/** Group an item kind belongs to. */
export function groupOfKind(kind: CalendarItemKind): CalendarKindGroup {
  return ALL_KIND_GROUPS.find((group) => KIND_GROUPS[group].includes(kind)) ?? ('events' as const);
}

/**
 * Combined calendar: the visible window (month, week, day or agenda) with events and derived dates.
 * Moving the window cancels the request in flight. Changes to events reload the window.
 */
@Injectable({ providedIn: 'root' })
export class CalendarV2Store {
  private readonly api = inject(CalendarV2ApiService);

  private readonly viewState = signal<CalendarViewMode>('month');
  private readonly anchorState = signal(startOfDay(new Date()));
  private readonly projectsState = signal<readonly string[]>([]);
  private readonly groupsState = signal<readonly CalendarKindGroup[]>(ALL_KIND_GROUPS);

  readonly window = new AsyncState<CalendarWindow>();
  readonly view = this.viewState.asReadonly();
  /** A day inside the visible window. */
  readonly anchor = this.anchorState.asReadonly();
  /** Projects the derived dates are limited to; empty means all the projects of the user. */
  readonly projects = this.projectsState.asReadonly();
  readonly groups = this.groupsState.asReadonly();
  readonly range = computed(() => viewRange(this.viewState(), this.anchorState()));

  readonly items = computed<readonly CalendarItem[]>(() => {
    const enabled = new Set(this.groupsState());
    return (this.window.value()?.items ?? []).filter((item) => enabled.has(groupOfKind(item.kind)));
  });
  readonly itemsByDay = computed(() => groupItemsByDay(this.items()));
  readonly truncated = computed(() => this.window.value()?.truncated ?? false);
  readonly isEmpty = computed(() => this.window.hasValue() && this.items().length === 0);

  readonly feed = new AsyncState<CalendarFeedState>();

  constructor() {
    resetOnLogout(() => this.reset());
  }

  /** Loads the window for the current view, anchor and project filter. */
  load(): void {
    const projects = this.projectsState();
    this.window.load(this.api.window(this.range(), { projects }));
  }

  setView(view: CalendarViewMode): void {
    if (view === this.viewState()) return;
    this.viewState.set(view);
    this.load();
  }

  setAnchor(anchor: Date): void {
    this.anchorState.set(startOfDay(anchor));
    this.load();
  }

  shift(steps: number): void {
    this.anchorState.update((anchor) => shiftAnchor(this.viewState(), anchor, steps));
    this.load();
  }

  today(): void {
    this.setAnchor(new Date());
  }

  setProjects(projects: readonly string[]): void {
    this.projectsState.set([...projects]);
    this.load();
  }

  toggleGroup(group: CalendarKindGroup): void {
    this.groupsState.update((current) =>
      current.includes(group) ? current.filter((entry) => entry !== group) : [...current, group],
    );
  }

  async create(input: CalendarEventInput): Promise<CalendarEventDetail> {
    const created = await firstValueFrom(this.api.create(input));
    this.load();
    return created;
  }

  async update(
    id: string,
    input: CalendarEventInput,
    occurrence?: string,
  ): Promise<CalendarEventDetail> {
    const updated = await firstValueFrom(this.api.update(id, input, occurrence));
    this.load();
    return updated;
  }

  async remove(id: string, occurrence?: string): Promise<void> {
    await firstValueFrom(this.api.remove(id, occurrence));
    this.load();
  }

  async respond(id: string, response: RsvpResponse): Promise<void> {
    await firstValueFrom(this.api.respond(id, response));
    this.load();
  }

  fetchEvent(id: string): Promise<CalendarEventDetail> {
    return firstValueFrom(this.api.get(id));
  }

  loadFeed(): void {
    this.feed.load(this.api.feed());
  }

  async createFeed(): Promise<CalendarFeedSecret> {
    const secret = await firstValueFrom(this.api.createFeed());
    this.feed.set({ enabled: secret.enabled, createdAt: secret.createdAt });
    return secret;
  }

  async revokeFeed(): Promise<void> {
    await firstValueFrom(this.api.revokeFeed());
    this.feed.set({ enabled: false, createdAt: null });
  }

  reset(): void {
    this.viewState.set('month');
    this.anchorState.set(startOfDay(new Date()));
    this.projectsState.set([]);
    this.groupsState.set(ALL_KIND_GROUPS);
    this.window.reset();
    this.feed.reset();
  }
}
