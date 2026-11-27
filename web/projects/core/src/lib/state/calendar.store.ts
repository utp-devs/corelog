import { Injectable, computed, inject, signal } from '@angular/core';

import type { CalendarEvent } from '../models/calendar.model';
import { CalendarApiService } from '../services/api/calendar-api.service';
import { monthRange, shiftMonth, startOfMonth } from '../utils/calendar-dates';
import { AsyncState } from './async-state';
import { resetOnLogout } from './session-scoped';

/** Events of one day, in the order the API returned them. */
export interface CalendarDay {
  readonly date: string;
  readonly events: readonly CalendarEvent[];
}

/**
 * Project calendar for one month at a time. The events are requested for the visible month; moving
 * to another month cancels the request in flight, so only the latest month is kept.
 */
@Injectable({ providedIn: 'root' })
export class CalendarStore {
  private readonly api = inject(CalendarApiService);
  private projectId: string | null = null;
  private readonly monthState = signal(startOfMonth(new Date()));

  readonly events = new AsyncState<CalendarEvent[]>();
  /** First day of the visible month. */
  readonly month = this.monthState.asReadonly();
  readonly range = computed(() => monthRange(this.monthState()));

  readonly eventsByDate = computed<Readonly<Record<string, readonly CalendarEvent[]>>>(() => {
    const byDate: Record<string, CalendarEvent[]> = {};
    for (const event of this.events.value() ?? []) {
      byDate[event.date] = [...(byDate[event.date] ?? []), event];
    }
    return byDate;
  });

  /** Days with events, in the order they were received. */
  readonly days = computed<readonly CalendarDay[]>(() =>
    Object.entries(this.eventsByDate()).map(([date, events]) => ({ date, events })),
  );

  readonly isEmpty = computed(
    () => this.events.hasValue() && (this.events.value()?.length ?? 0) === 0,
  );

  constructor() {
    resetOnLogout(() => this.reset());
  }

  /** Loads the visible month of a project. */
  load(projectId: string): void {
    this.projectId = projectId;
    this.fetch();
  }

  /** Moves the visible month by `months` and reloads it. */
  shift(months: number): void {
    this.monthState.update((current) => shiftMonth(current, months));
    this.fetch();
  }

  retry(): void {
    this.fetch();
  }

  reset(): void {
    this.projectId = null;
    this.monthState.set(startOfMonth(new Date()));
    this.events.reset();
  }

  private fetch(): void {
    const projectId = this.projectId;
    if (!projectId) {
      return;
    }
    this.events.load(this.api.events(projectId, this.range()));
  }
}
