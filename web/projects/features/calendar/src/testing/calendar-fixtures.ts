import type { CalendarEventDetail, CalendarItem } from '@corelog/core';

export function item(patch: Partial<CalendarItem> = {}): CalendarItem {
  return {
    kind: 'event',
    title: 'Reunión',
    startsAt: '2026-11-25T10:00:00',
    endsAt: '2026-11-25T11:00:00',
    allDay: false,
    date: '2026-11-25',
    sourceId: 'ev-1',
    projectId: null,
    projectKey: null,
    link: '',
    color: 'blue',
    location: '',
    timezone: 'UTC',
    visibility: 'private',
    recurring: false,
    occurrenceStart: null,
    canEdit: true,
    myResponse: null,
    ...patch,
  };
}

export function detail(patch: Partial<CalendarEventDetail> = {}): CalendarEventDetail {
  return {
    id: 'ev-1',
    title: 'Reunión',
    description: '',
    location: '',
    startsAt: '2026-11-25T10:00:00Z',
    endsAt: '2026-11-25T11:00:00Z',
    allDay: false,
    timezone: 'UTC',
    recurrenceRule: null,
    recurrenceExceptions: [],
    visibility: 'private',
    color: 'blue',
    projectId: null,
    attendees: [],
    myResponse: null,
    canEdit: true,
    ...patch,
  };
}
