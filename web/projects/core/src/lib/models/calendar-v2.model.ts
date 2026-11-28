/** What a calendar item represents. Events are editable; the rest are derived from planning data. */
export type CalendarItemKind =
  'event' | 'issue_due' | 'issue_start' | 'sprint_start' | 'sprint_end' | 'version_release';

export type EventVisibility = 'private' | 'project' | 'organization';
export type RsvpResponse = 'needs_action' | 'accepted' | 'declined' | 'tentative';

/** Color keys accepted for an event. */
export const EVENT_COLORS = [
  'gray',
  'red',
  'orange',
  'amber',
  'green',
  'teal',
  'blue',
  'indigo',
  'purple',
  'pink',
] as const;
export type EventColor = (typeof EVENT_COLORS)[number];

/** One entry of the combined calendar: an event occurrence or a derived date. */
export interface CalendarItem {
  readonly kind: CalendarItemKind;
  readonly title: string;
  /** Start instant, RFC 3339. */
  readonly startsAt: string;
  /** End instant, RFC 3339 (exclusive for all-day items). */
  readonly endsAt: string;
  readonly allDay: boolean;
  /** Day of the start, AAAA-MM-DD. */
  readonly date: string;
  readonly sourceId: string;
  readonly projectId: string | null;
  readonly projectKey: string | null;
  /** Route of the source (an issue, a sprint list). */
  readonly link: string;
  readonly color: string;
  readonly location: string;
  readonly timezone: string;
  readonly visibility: EventVisibility | '';
  readonly recurring: boolean;
  /** Original instant of this occurrence inside its series. */
  readonly occurrenceStart: string | null;
  readonly canEdit: boolean;
  readonly myResponse: RsvpResponse | null;
}

export interface CalendarWindow {
  readonly items: readonly CalendarItem[];
  readonly truncated: boolean;
}

export interface CalendarAttendee {
  readonly userId: string;
  readonly name: string;
  readonly response: RsvpResponse;
}

export interface CalendarEventDetail {
  readonly id: string;
  readonly title: string;
  readonly description: string;
  readonly location: string;
  readonly startsAt: string;
  readonly endsAt: string;
  readonly allDay: boolean;
  readonly timezone: string;
  readonly recurrenceRule: string | null;
  readonly recurrenceExceptions: readonly string[];
  readonly visibility: EventVisibility;
  readonly color: string;
  readonly projectId: string | null;
  readonly attendees: readonly CalendarAttendee[];
  readonly myResponse: RsvpResponse | null;
  readonly canEdit: boolean;
}

/** Fields of a new event or of a change. In a change only the present fields are sent. */
export interface CalendarEventInput {
  readonly title?: string;
  readonly description?: string;
  readonly location?: string;
  readonly startsAt?: string;
  readonly endsAt?: string;
  readonly allDay?: boolean;
  readonly timezone?: string;
  readonly recurrenceRule?: string | null;
  readonly visibility?: EventVisibility;
  readonly color?: string;
  readonly projectId?: string | null;
  readonly organizationId?: string | null;
  readonly attendees?: readonly string[];
}

export interface CalendarFeedState {
  readonly enabled: boolean;
  readonly createdAt: string | null;
}

/** The token and URL are shown a single time, when the feed is created or rotated. */
export interface CalendarFeedSecret extends CalendarFeedState {
  readonly token: string;
  readonly url: string;
}

export type CalendarViewMode = 'month' | 'week' | 'day' | 'agenda';
