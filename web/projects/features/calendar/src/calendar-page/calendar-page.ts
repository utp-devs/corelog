import { ChangeDetectionStrategy, Component, computed, inject } from '@angular/core';
import { CalendarV2Store } from '@corelog/core';
import { PageContainer } from '@corelog/ui/page-container';
import { PageHeader } from '@corelog/ui/page-header';

import { CalendarView } from '../calendar-view/calendar-view';

/** Calendar of the signed-in user: events plus the dates of issues, sprints and versions of their projects. */
@Component({
  selector: 'app-calendar-page',
  imports: [CalendarView, PageContainer, PageHeader],
  templateUrl: './calendar-page.html',
  styleUrl: './calendar-page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class CalendarPage {
  /** The month, week and day views fill the page height; the agenda is as tall as its list. */
  private readonly calendar = inject(CalendarV2Store);
  protected readonly grid = computed(() => this.calendar.view() !== 'agenda');
}
