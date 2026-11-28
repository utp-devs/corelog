import {
  ChangeDetectionStrategy,
  Component,
  DestroyRef,
  ElementRef,
  Injector,
  afterNextRender,
  computed,
  inject,
  input,
  output,
  signal,
} from '@angular/core';
import {
  fullDateLabel,
  isoDate,
  isTimedItem,
  layoutTimedItems,
  viewDays,
  type CalendarItem,
  type CalendarViewMode,
} from '@corelog/core';

import { KIND_META, itemAriaLabel, itemColor, timeLabel } from '../calendar-meta';

const HOURS = Array.from({ length: 24 }, (_, hour) => hour);
const FIRST_VISIBLE_HOUR = 7;

interface Column {
  readonly iso: string;
  readonly date: Date;
  readonly allDay: readonly CalendarItem[];
  readonly placements: readonly {
    readonly item: CalendarItem;
    readonly top: number;
    readonly height: number;
    readonly left: number;
    readonly width: number;
  }[];
}

/**
 * Week and day views: a header with the days, a row for all-day items and a 24 hour column per day with the
 * timed items placed by their time. Overlapping items sit side by side. Clicking an empty slot starts an event.
 */
@Component({
  selector: 'app-calendar-time-grid',
  templateUrl: './calendar-time-grid.html',
  styleUrl: './calendar-time-grid.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class CalendarTimeGrid {
  private readonly host = inject<ElementRef<HTMLElement>>(ElementRef);
  private readonly injector = inject(Injector);

  readonly anchor = input.required<Date>();
  readonly mode = input<Extract<CalendarViewMode, 'week' | 'day'>>('week');
  readonly itemsByDay = input.required<Readonly<Record<string, readonly CalendarItem[]>>>();

  readonly itemSelected = output<CalendarItem>();
  /** An empty slot was clicked: the start of that hour (or the day for the all-day row). */
  readonly slotActivated = output<{ readonly date: Date; readonly allDay: boolean }>();
  readonly dayOpened = output<Date>();

  protected readonly hours = HOURS;
  protected readonly today = isoDate(new Date());
  protected readonly now = signal(new Date());
  protected readonly kindMeta = KIND_META;

  protected readonly columns = computed<Column[]>(() =>
    viewDays(this.mode(), this.anchor()).map((date) => {
      const iso = isoDate(date);
      const items = this.itemsByDay()[iso] ?? [];
      const timed = items.filter(isTimedItem);
      return {
        iso,
        date,
        allDay: items.filter((item) => !isTimedItem(item)),
        placements: layoutTimedItems(timed).map((placement) => ({
          item: placement.item,
          top: (placement.startMinute / 1440) * 100,
          height: (placement.durationMinutes / 1440) * 100,
          left: (placement.lane / placement.lanes) * 100,
          width: 100 / placement.lanes,
        })),
      };
    }),
  );

  protected readonly nowPercent = computed(() => {
    const now = this.now();
    return ((now.getHours() * 60 + now.getMinutes()) / 1440) * 100;
  });

  constructor() {
    afterNextRender(
      () => {
        const scroller = this.host.nativeElement.querySelector<HTMLElement>(
          '.calendar-time-grid__scroll',
        );
        if (scroller)
          scroller.scrollTop = Math.max(0, (scroller.scrollHeight / 24) * FIRST_VISIBLE_HOUR - 8);
      },
      { injector: this.injector },
    );
    const timer = setInterval(() => this.now.set(new Date()), 60_000);
    inject(DestroyRef).onDestroy(() => clearInterval(timer));
  }

  protected headerLabel(date: Date): string {
    return fullDateLabel(date);
  }

  protected weekday(date: Date): string {
    return date
      .toLocaleDateString('es-ES', { weekday: 'short' })
      .replace('.', '')
      .replace(/^./, (letter) => letter.toUpperCase());
  }

  protected color(item: CalendarItem): string {
    return itemColor(item);
  }

  protected label(item: CalendarItem): string {
    return itemAriaLabel(item);
  }

  protected time(item: CalendarItem): string {
    return timeLabel(item);
  }

  protected hourLabel(hour: number): string {
    return `${String(hour).padStart(2, '0')}:00`;
  }

  protected onSlotClick(event: MouseEvent, date: Date): void {
    if ((event.target as HTMLElement).closest('button')) return;
    const column = event.currentTarget as HTMLElement;
    const rect = column.getBoundingClientRect();
    const ratio = rect.height > 0 ? (event.clientY - rect.top) / rect.height : 0;
    const hour = Math.min(23, Math.max(0, Math.floor(ratio * 24)));
    this.slotActivated.emit({
      date: new Date(date.getFullYear(), date.getMonth(), date.getDate(), hour),
      allDay: false,
    });
  }

  protected onAllDayClick(event: MouseEvent, date: Date): void {
    if ((event.target as HTMLElement).closest('button')) return;
    this.slotActivated.emit({ date, allDay: true });
  }
}
