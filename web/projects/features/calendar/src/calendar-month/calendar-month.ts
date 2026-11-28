import {
  ChangeDetectionStrategy,
  Component,
  ElementRef,
  computed,
  inject,
  input,
  output,
  signal,
} from '@angular/core';
import { addDays, fullDateLabel, isoDate, monthGridDays, type CalendarItem } from '@corelog/core';

import { KIND_META, itemAriaLabel, itemColor, timeLabel } from '../calendar-meta';

const WEEKDAY_HEADERS = ['Lun', 'Mar', 'Mié', 'Jue', 'Vie', 'Sáb', 'Dom'];
const MAX_CHIPS = 3;

/**
 * Month grid with weeks starting on Monday. The grid is a single tab stop: arrow keys move between days,
 * Enter starts a new event on the focused day and the chips inside a day are regular buttons.
 */
@Component({
  selector: 'app-calendar-month',
  templateUrl: './calendar-month.html',
  styleUrl: './calendar-month.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class CalendarMonth {
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);

  /** Any day of the month to show. */
  readonly anchor = input.required<Date>();
  readonly itemsByDay = input.required<Readonly<Record<string, readonly CalendarItem[]>>>();

  readonly itemSelected = output<CalendarItem>();
  /** A day was activated to create an event on it. */
  readonly dayActivated = output<Date>();
  /** The "+N" link or the day number asks for the day view. */
  readonly dayOpened = output<Date>();
  /** Arrow keys left the visible weeks: the host moves the window. */
  readonly outOfRange = output<Date>();

  protected readonly headers = WEEKDAY_HEADERS;
  protected readonly today = isoDate(new Date());
  protected readonly focusedIso = signal<string | null>(null);

  protected readonly weeks = computed(() => {
    const days = monthGridDays(this.anchor());
    const month = this.anchor().getMonth();
    const weeks: { iso: string; date: Date; inMonth: boolean }[][] = [];
    for (let index = 0; index < days.length; index += 7) {
      weeks.push(
        days.slice(index, index + 7).map((date) => ({
          iso: isoDate(date),
          date,
          inMonth: date.getMonth() === month,
        })),
      );
    }
    return weeks;
  });

  /** The day that holds the tab stop: the focused one, today or the first of the month. */
  protected readonly tabStop = computed(() => {
    const focused = this.focusedIso();
    const all = this.weeks().flat();
    if (focused && all.some((day) => day.iso === focused)) return focused;
    if (all.some((day) => day.iso === this.today)) return this.today;
    return all.find((day) => day.inMonth)?.iso ?? all[0].iso;
  });

  protected readonly maxChips = MAX_CHIPS;
  protected readonly kindMeta = KIND_META;

  protected itemsOf(iso: string): readonly CalendarItem[] {
    return this.itemsByDay()[iso] ?? [];
  }

  protected color(item: CalendarItem): string {
    return itemColor(item);
  }

  protected label(item: CalendarItem): string {
    return itemAriaLabel(item);
  }

  protected time(item: CalendarItem): string {
    return item.allDay ? '' : timeLabel(item).split(' - ')[0];
  }

  protected dayLabel(date: Date, count: number): string {
    const base = fullDateLabel(date);
    return count === 0 ? base : `${base}, ${count} ${count === 1 ? 'elemento' : 'elementos'}`;
  }

  protected onKeydown(event: KeyboardEvent, date: Date): void {
    // Keys typed inside a chip keep their own meaning.
    if ((event.target as HTMLElement).closest('.calendar-month__chip, .calendar-month__more'))
      return;
    const steps: Record<string, number> = {
      ArrowLeft: -1,
      ArrowRight: 1,
      ArrowUp: -7,
      ArrowDown: 7,
    };
    if (event.key in steps) {
      event.preventDefault();
      this.moveFocus(addDays(date, steps[event.key]));
    } else if (event.key === 'Home' || event.key === 'End') {
      event.preventDefault();
      const monday = addDays(date, -((date.getDay() + 6) % 7));
      this.moveFocus(event.key === 'Home' ? monday : addDays(monday, 6));
    } else if (event.key === 'Enter' || event.key === ' ') {
      event.preventDefault();
      this.dayActivated.emit(date);
    }
  }

  protected onCellClick(event: MouseEvent, date: Date): void {
    if ((event.target as HTMLElement).closest('button')) return;
    this.focusedIso.set(isoDate(date));
    this.dayActivated.emit(date);
  }

  private moveFocus(target: Date): void {
    const iso = isoDate(target);
    this.focusedIso.set(iso);
    if (
      !this.weeks()
        .flat()
        .some((day) => day.iso === iso)
    ) {
      this.outOfRange.emit(target);
    }
    // The cell may be drawn after the window moves: focus once it exists.
    queueMicrotask(() => this.focusCell(iso));
    setTimeout(() => this.focusCell(iso), 150);
  }

  private focusCell(iso: string): void {
    this.host.nativeElement.querySelector<HTMLElement>(`[data-day="${iso}"]`)?.focus();
  }
}
