import {
  ChangeDetectionStrategy,
  Component,
  computed,
  effect,
  inject,
  signal,
} from '@angular/core';
import { RouterLink } from '@angular/router';
import { Timesheet, WorklogApiService, asApiError, formatDuration, issuePath } from '@corelog/core';
import { EmptyState } from '@corelog/ui/empty-state';
import { LoadingSkeleton } from '@corelog/ui/loading-skeleton';
import { PageContainer } from '@corelog/ui/page-container';
import { PageHeader } from '@corelog/ui/page-header';
import { ProgressBar } from '@corelog/ui/progress-bar';
import { Button } from 'primeng/button';
import { Message } from 'primeng/message';
import { firstValueFrom } from 'rxjs';

import { buildTimesheetGrid, shiftWeeks, toIsoDate, weekDays } from './timesheet-grid';

const DAY_NAME = new Intl.DateTimeFormat('es', {
  weekday: 'short',
  day: 'numeric',
  month: 'short',
});

/**
 * Personal timesheet: the time the user logged in one week, one row per issue and one column per
 * day, with the total of each day against the daily capacity of the organization.
 */
@Component({
  selector: 'app-timesheet-page',
  imports: [
    Button,
    EmptyState,
    LoadingSkeleton,
    Message,
    PageContainer,
    PageHeader,
    ProgressBar,
    RouterLink,
  ],
  templateUrl: './timesheet-page.html',
  styleUrl: './timesheet-page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class TimesheetPage {
  private readonly api = inject(WorklogApiService);

  protected readonly reference = signal(new Date());
  protected readonly sheet = signal<Timesheet | null>(null);
  protected readonly loading = signal(true);
  protected readonly error = signal<string | null>(null);

  protected readonly days = computed(() => weekDays(this.reference()));
  protected readonly grid = computed(() => buildTimesheetGrid(this.sheet(), this.days()));
  protected readonly rangeLabel = computed(() => {
    const days = this.days();
    return `${this.label(days[0])} - ${this.label(days[6])}`;
  });
  protected readonly isCurrentWeek = computed(() => this.days()[0] === weekDays(new Date())[0]);
  protected readonly today = toIsoDate(new Date());

  protected readonly format = (seconds: number): string =>
    seconds > 0 ? formatDuration(seconds) : '';
  protected readonly pathOf = issuePath;

  constructor() {
    effect(() => {
      const days = this.days();
      void this.load(days[0], days[6]);
    });
  }

  protected label(iso: string): string {
    return DAY_NAME.format(new Date(`${iso}T00:00:00`));
  }

  protected percent(total: number, capacity: number): number {
    return capacity > 0 ? (total / capacity) * 100 : 0;
  }

  protected tone(total: number, capacity: number): 'brand' | 'success' | 'warning' {
    if (capacity <= 0 || total === 0) return 'brand';
    return total > capacity ? 'warning' : total === capacity ? 'success' : 'brand';
  }

  protected previous(): void {
    this.reference.update((date) => shiftWeeks(date, -1));
  }

  protected next(): void {
    this.reference.update((date) => shiftWeeks(date, 1));
  }

  protected current(): void {
    this.reference.set(new Date());
  }

  private async load(from: string, to: string): Promise<void> {
    this.loading.set(true);
    this.error.set(null);
    try {
      const sheet = await firstValueFrom(this.api.timesheet(from, to));
      if (from === this.days()[0]) this.sheet.set(sheet);
    } catch (error) {
      this.error.set(asApiError(error).message);
      this.sheet.set(null);
    } finally {
      this.loading.set(false);
    }
  }
}
