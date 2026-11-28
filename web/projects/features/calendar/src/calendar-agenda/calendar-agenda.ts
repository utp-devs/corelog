import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';
import { isoDate, parseIsoDate, type CalendarItem } from '@corelog/core';

import { KIND_META, itemColor, timeLabel } from '../calendar-meta';

/** Items of the visible month as a list grouped by day: the view for phones and for scanning what is next. */
@Component({
  selector: 'app-calendar-agenda',
  templateUrl: './calendar-agenda.html',
  styleUrl: './calendar-agenda.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class CalendarAgenda {
  readonly anchor = input.required<Date>();
  readonly itemsByDay = input.required<Readonly<Record<string, readonly CalendarItem[]>>>();
  readonly itemSelected = output<CalendarItem>();

  protected readonly today = isoDate(new Date());
  protected readonly kindMeta = KIND_META;

  protected readonly days = computed(() => {
    const anchor = this.anchor();
    return Object.entries(this.itemsByDay())
      .filter(([iso]) => {
        const date = parseIsoDate(iso);
        return date.getMonth() === anchor.getMonth() && date.getFullYear() === anchor.getFullYear();
      })
      .sort(([a], [b]) => a.localeCompare(b))
      .map(([iso, items]) => ({ iso, date: parseIsoDate(iso), items }));
  });

  protected dayTitle(date: Date): string {
    const text = date.toLocaleDateString('es-ES', {
      weekday: 'long',
      day: 'numeric',
      month: 'long',
    });
    return text.charAt(0).toUpperCase() + text.slice(1);
  }

  protected color(item: CalendarItem): string {
    return itemColor(item);
  }

  protected time(item: CalendarItem): string {
    return timeLabel(item);
  }
}
