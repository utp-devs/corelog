/** Milestone kind shown in the project calendar. */
export type CalendarEventKind = 'sprint_start' | 'sprint_end' | 'issue_due';

export interface CalendarEvent {
  readonly kind: CalendarEventKind;
  /** Day of the event, AAAA-MM-DD. */
  readonly date: string;
  readonly title: string;
  readonly sourceId: string;
  readonly link: string;
}

/** Inclusive date range requested from the calendar, both ends AAAA-MM-DD. */
export interface CalendarRange {
  readonly from: string;
  readonly to: string;
}
