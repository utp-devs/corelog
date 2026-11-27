import { HttpClient } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable, map } from 'rxjs';

import { API_URL } from '../../config/api.config';
import type { CalendarEvent, CalendarEventKind, CalendarRange } from '../../models/calendar.model';
import { silentRequest } from '../../utils/silent-request';

interface CalendarEventDto {
  readonly kind: CalendarEventKind;
  readonly date: string;
  readonly title: string;
  readonly source_id: string;
  readonly link: string;
}

interface DataEnvelope<T> {
  readonly data: T;
}

/** Milestones of a project (sprint boundaries and issue due dates) within a date range. */
@Injectable({ providedIn: 'root' })
export class CalendarApiService {
  private readonly http = inject(HttpClient);
  private readonly baseUrl = inject(API_URL);

  events(projectId: string, range: CalendarRange): Observable<CalendarEvent[]> {
    return this.http
      .get<DataEnvelope<CalendarEventDto[]>>(`${this.baseUrl}/calendar/events`, {
        params: { project_id: projectId, from: range.from, to: range.to },
        context: silentRequest(),
      })
      .pipe(
        map((response) =>
          response.data.map((event) => ({
            kind: event.kind,
            date: event.date,
            title: event.title,
            sourceId: event.source_id,
            link: event.link,
          })),
        ),
      );
  }
}
