import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';

import { provideCorelogTesting } from '../../../testing';
import type { CalendarEvent } from '../../models/calendar.model';
import { CalendarApiService } from './calendar-api.service';

function setup(): { service: CalendarApiService; backend: HttpTestingController } {
  TestBed.configureTestingModule({
    providers: [provideCorelogTesting(), provideHttpClient(), provideHttpClientTesting()],
  });
  return {
    service: TestBed.inject(CalendarApiService),
    backend: TestBed.inject(HttpTestingController),
  };
}

describe('CalendarApiService', () => {
  it('requests the range with dates and maps the events', () => {
    const { service, backend } = setup();
    let events: CalendarEvent[] = [];

    service
      .events('project-1', { from: '2026-11-01', to: '2026-11-30' })
      .subscribe((value) => (events = value));
    backend
      .expectOne('/api/v1/calendar/events?project_id=project-1&from=2026-11-01&to=2026-11-30')
      .flush({
        data: [
          {
            kind: 'issue_due',
            date: '2026-11-05',
            title: 'Entregar',
            source_id: 'issue-1',
            link: '/issues/issue-1',
          },
        ],
      });

    expect(events).toEqual([
      {
        kind: 'issue_due',
        date: '2026-11-05',
        title: 'Entregar',
        sourceId: 'issue-1',
        link: '/issues/issue-1',
      },
    ]);
    backend.verify();
  });
});
