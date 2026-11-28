import { ChangeDetectionStrategy, Component, computed, inject } from '@angular/core';
import { CalendarV2Store } from '@corelog/core';
import { CalendarView } from '@corelog/features/calendar';
import { PageContainer } from '@corelog/ui/page-container';
import { PageHeader } from '@corelog/ui/page-header';

import { injectProjectId } from '../shared/project-route';

/** Calendar of one project: its events and the dates of its issues, sprints and versions. */
@Component({
  selector: 'app-project-calendar-page',
  imports: [CalendarView, PageContainer, PageHeader],
  templateUrl: './project-calendar-page.html',
  styleUrl: './project-calendar-page.scss',
  changeDetection: ChangeDetectionStrategy.OnPush,
})
export class ProjectCalendarPage {
  protected readonly projectId = injectProjectId();
  /** The month, week and day views fill the page height; the agenda is as tall as its list. */
  private readonly calendar = inject(CalendarV2Store);
  protected readonly grid = computed(() => this.calendar.view() !== 'agenda');
}
