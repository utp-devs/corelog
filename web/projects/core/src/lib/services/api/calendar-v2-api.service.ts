import { HttpClient, HttpParams } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable, map } from 'rxjs';

import { API_URL } from '../../config/api.config';
import { normalizeIssueLink } from '../../config/issue-paths';
import type {
  CalendarAttendee,
  CalendarEventDetail,
  CalendarEventInput,
  CalendarFeedSecret,
  CalendarFeedState,
  CalendarItem,
  CalendarItemKind,
  CalendarWindow,
  EventVisibility,
  RsvpResponse,
} from '../../models/calendar-v2.model';
import type { CalendarRange } from '../../models/calendar.model';
import { silentRequest } from '../../utils/silent-request';

interface ItemDto {
  readonly kind: CalendarItemKind;
  readonly title: string;
  readonly starts_at: string;
  readonly ends_at: string;
  readonly all_day: boolean;
  readonly date: string;
  readonly source_id: string;
  readonly project_id?: string | null;
  readonly project_key?: string | null;
  readonly link?: string;
  readonly color?: string;
  readonly location?: string;
  readonly timezone?: string;
  readonly visibility?: EventVisibility | '';
  readonly recurring?: boolean;
  readonly occurrence_start?: string | null;
  readonly can_edit?: boolean;
  readonly my_response?: RsvpResponse | null;
}

interface AttendeeDto {
  readonly user_id: string;
  readonly name: string;
  readonly response: RsvpResponse;
}

interface EventDto {
  readonly id: string;
  readonly title: string;
  readonly description?: string;
  readonly location?: string;
  readonly starts_at: string;
  readonly ends_at: string;
  readonly all_day: boolean;
  readonly timezone?: string;
  readonly recurrence_rule?: string | null;
  readonly recurrence_exceptions?: readonly string[] | null;
  readonly visibility: EventVisibility;
  readonly color?: string;
  readonly project_id?: string | null;
  readonly attendees?: readonly AttendeeDto[] | null;
  readonly my_response?: RsvpResponse | null;
  readonly can_edit?: boolean;
}

interface FeedDto {
  readonly enabled: boolean;
  readonly created_at: string | null;
  readonly token?: string;
  readonly url?: string;
}

interface Envelope<T> {
  readonly data: T;
}

function toItem(dto: ItemDto): CalendarItem {
  return {
    kind: dto.kind,
    title: dto.title,
    startsAt: dto.starts_at,
    endsAt: dto.ends_at,
    allDay: dto.all_day,
    date: dto.date,
    sourceId: dto.source_id,
    projectId: dto.project_id ?? null,
    projectKey: dto.project_key ?? null,
    link: normalizeIssueLink(dto.link ?? ''),
    color: dto.color ?? '',
    location: dto.location ?? '',
    timezone: dto.timezone ?? '',
    visibility: dto.visibility ?? '',
    recurring: dto.recurring ?? false,
    occurrenceStart: dto.occurrence_start ?? null,
    canEdit: dto.can_edit ?? false,
    myResponse: dto.my_response ?? null,
  };
}

function toEvent(dto: EventDto): CalendarEventDetail {
  return {
    id: dto.id,
    title: dto.title,
    description: dto.description ?? '',
    location: dto.location ?? '',
    startsAt: dto.starts_at,
    endsAt: dto.ends_at,
    allDay: dto.all_day,
    timezone: dto.timezone ?? '',
    recurrenceRule: dto.recurrence_rule ?? null,
    recurrenceExceptions: dto.recurrence_exceptions ?? [],
    visibility: dto.visibility,
    color: dto.color ?? '',
    projectId: dto.project_id ?? null,
    attendees: (dto.attendees ?? []).map((attendee): CalendarAttendee => ({
      userId: attendee.user_id,
      name: attendee.name,
      response: attendee.response,
    })),
    myResponse: dto.my_response ?? null,
    canEdit: dto.can_edit ?? false,
  };
}

function toBody(input: CalendarEventInput): Record<string, unknown> {
  const body: Record<string, unknown> = {};
  const set = (key: string, value: unknown): void => {
    if (value !== undefined) body[key] = value;
  };
  set('title', input.title);
  set('description', input.description);
  set('location', input.location);
  set('starts_at', input.startsAt);
  set('ends_at', input.endsAt);
  set('all_day', input.allDay);
  set('timezone', input.timezone);
  set('recurrence_rule', input.recurrenceRule);
  set('visibility', input.visibility);
  set('color', input.color);
  set('project_id', input.projectId);
  set('organization_id', input.organizationId);
  set('attendees', input.attendees);
  return body;
}

function toFeed(dto: FeedDto): CalendarFeedState {
  return { enabled: dto.enabled, createdAt: dto.created_at };
}

/** Combined calendar (events with their occurrences plus derived dates), events and the ICS feed. */
@Injectable({ providedIn: 'root' })
export class CalendarV2ApiService {
  private readonly http = inject(HttpClient);
  private readonly baseUrl = inject(API_URL);

  window(
    range: CalendarRange,
    options: { readonly projects?: readonly string[]; readonly types?: readonly string[] } = {},
  ): Observable<CalendarWindow> {
    let params = new HttpParams().set('from', range.from).set('to', range.to);
    if (options.projects?.length) params = params.set('projects', options.projects.join(','));
    if (options.types?.length) params = params.set('types', options.types.join(','));
    return this.http
      .get<Envelope<{ items: ItemDto[] | null; truncated: boolean }>>(`${this.baseUrl}/calendar`, {
        params,
        context: silentRequest(),
      })
      .pipe(
        map(({ data }) => ({
          items: (data.items ?? []).map(toItem),
          truncated: data.truncated,
        })),
      );
  }

  get(id: string): Observable<CalendarEventDetail> {
    return this.http
      .get<Envelope<EventDto>>(`${this.baseUrl}/calendar/events/${encodeURIComponent(id)}`, {
        context: silentRequest(),
      })
      .pipe(map(({ data }) => toEvent(data)));
  }

  create(input: CalendarEventInput): Observable<CalendarEventDetail> {
    return this.http
      .post<Envelope<EventDto>>(`${this.baseUrl}/calendar/events`, toBody(input), {
        context: silentRequest(),
      })
      .pipe(map(({ data }) => toEvent(data)));
  }

  /** Changes the whole event, or only one occurrence when `occurrence` (its original instant) is given. */
  update(
    id: string,
    input: CalendarEventInput,
    occurrence?: string,
  ): Observable<CalendarEventDetail> {
    const params = occurrence ? new HttpParams().set('occurrence', occurrence) : undefined;
    return this.http
      .patch<Envelope<EventDto>>(
        `${this.baseUrl}/calendar/events/${encodeURIComponent(id)}`,
        toBody(input),
        { params, context: silentRequest() },
      )
      .pipe(map(({ data }) => toEvent(data)));
  }

  /** Deletes the event, or cancels one occurrence when `occurrence` is given. */
  remove(id: string, occurrence?: string): Observable<void> {
    const params = occurrence ? new HttpParams().set('occurrence', occurrence) : undefined;
    return this.http.delete<void>(`${this.baseUrl}/calendar/events/${encodeURIComponent(id)}`, {
      params,
      context: silentRequest(),
    });
  }

  respond(id: string, response: RsvpResponse): Observable<void> {
    return this.http
      .put<unknown>(
        `${this.baseUrl}/calendar/events/${encodeURIComponent(id)}/attendees/me`,
        { response },
        { context: silentRequest() },
      )
      .pipe(map(() => undefined));
  }

  feed(): Observable<CalendarFeedState> {
    return this.http
      .get<Envelope<FeedDto>>(`${this.baseUrl}/me/calendar/feed`, { context: silentRequest() })
      .pipe(map(({ data }) => toFeed(data)));
  }

  /** Creates or rotates the feed. The token and URL are only returned here. */
  createFeed(): Observable<CalendarFeedSecret> {
    return this.http
      .post<Envelope<FeedDto>>(`${this.baseUrl}/me/calendar/feed`, {}, { context: silentRequest() })
      .pipe(map(({ data }) => ({ ...toFeed(data), token: data.token ?? '', url: data.url ?? '' })));
  }

  revokeFeed(): Observable<void> {
    return this.http.delete<void>(`${this.baseUrl}/me/calendar/feed`, { context: silentRequest() });
  }
}
