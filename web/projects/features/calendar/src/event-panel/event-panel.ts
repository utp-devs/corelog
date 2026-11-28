import { ChangeDetectionStrategy, Component, computed, input, output } from '@angular/core';
import {
  parseRecurrence,
  summarizeRecurrence,
  type CalendarEventDetail,
  type CalendarItem,
  type RsvpResponse,
} from '@corelog/core';
import { SidePanel } from '@corelog/ui/side-panel';
import { Button } from 'primeng/button';

import { RSVP_LABEL, VISIBILITY_LABEL } from '../calendar-meta';

const RESPONSES: readonly RsvpResponse[] = ['accepted', 'tentative', 'declined'];

/** Details of one event occurrence: when, where, guests and their answers, with the actions the person may take. */
@Component({
  selector: 'app-event-panel',
  imports: [Button, SidePanel],
  templateUrl: './event-panel.html',
  styleUrl: './event-panel.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class EventPanel {
  readonly open = input(false);
  /** The occurrence that was clicked. */
  readonly item = input<CalendarItem | null>(null);
  readonly detail = input<CalendarEventDetail | null>(null);
  readonly loading = input(false);
  readonly error = input<string | null>(null);
  readonly busy = input(false);

  readonly edit = output<void>();
  readonly remove = output<void>();
  readonly respond = output<RsvpResponse>();
  readonly closeRequested = output<void>();

  protected readonly responses = RESPONSES;
  protected readonly responseLabel = RSVP_LABEL;
  protected readonly visibilityLabel = VISIBILITY_LABEL;

  protected readonly heading = computed(() => this.item()?.title ?? 'Evento');
  protected readonly recurrence = computed(() => {
    const rule = this.detail()?.recurrenceRule;
    return rule ? summarizeRecurrence(parseRecurrence(rule)) : null;
  });

  /** "viernes, 27 de noviembre · 10:00 - 11:00" or the day span for all-day items. */
  protected readonly when = computed(() => {
    const item = this.item();
    if (!item) return '';
    const day = (iso: string, withYear: boolean): string =>
      new Date(iso).toLocaleDateString('es-ES', {
        weekday: 'long',
        day: 'numeric',
        month: 'long',
        ...(withYear ? { year: 'numeric' as const } : {}),
      });
    if (item.allDay) {
      const start = new Date(`${item.date}T00:00:00`);
      const lastDay = new Date(item.endsAt);
      const last = new Date(
        lastDay.getUTCFullYear(),
        lastDay.getUTCMonth(),
        lastDay.getUTCDate() - 1,
      );
      const text = start.toLocaleDateString('es-ES', {
        weekday: 'long',
        day: 'numeric',
        month: 'long',
        year: 'numeric',
      });
      return last > start
        ? `${text} - ${last.toLocaleDateString('es-ES', { day: 'numeric', month: 'long' })} (todo el día)`
        : `${text} (todo el día)`;
    }
    const time = (iso: string): string =>
      new Date(iso).toLocaleTimeString('es-ES', { hour: '2-digit', minute: '2-digit' });
    return `${day(item.startsAt, true)} · ${time(item.startsAt)} - ${time(item.endsAt)}`;
  });
}
