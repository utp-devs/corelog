export type CalendarEventKind = 'sprint_start' | 'sprint_end' | 'issue_due'

export interface CalendarEvent {
  kind: CalendarEventKind
  date: string
  title: string
  sourceId: string
  link: string
}
