import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';

import { provideCorelogTesting } from '../../testing';
import { CalendarV2Store } from './calendar-v2.store';

function dto(kind: string, date: string) {
  return {
    kind,
    title: kind,
    starts_at: `${date}T10:00:00Z`,
    ends_at: `${date}T11:00:00Z`,
    all_day: false,
    date,
    source_id: kind,
  };
}

describe('CalendarV2Store', () => {
  function setup() {
    TestBed.configureTestingModule({
      providers: [provideCorelogTesting(), provideHttpClient(), provideHttpClientTesting()],
    });
    return {
      store: TestBed.inject(CalendarV2Store),
      backend: TestBed.inject(HttpTestingController),
    };
  }

  it('requests the visible range with the project filter and filters kinds locally', () => {
    const { store, backend } = setup();
    store.setProjects(['p1', 'p2']);
    const request = backend.expectOne((r) => r.url.endsWith('/calendar'));
    expect(request.request.params.get('projects')).toBe('p1,p2');
    expect(request.request.params.get('from')).toBe(store.range().from);
    request.flush({
      data: {
        items: [dto('event', '2026-11-25'), dto('issue_due', '2026-11-25')],
        truncated: false,
      },
    });
    expect(store.items().length).toBe(2);
    store.toggleGroup('issues');
    expect(store.items().map((i) => i.kind)).toEqual(['event']);
    expect(store.truncated()).toBe(false);
  });

  it('reloads after moving the window and answers invitations', () => {
    const { store, backend } = setup();
    store.load();
    backend
      .expectOne((r) => r.url.endsWith('/calendar'))
      .flush({ data: { items: [], truncated: false } });
    expect(store.isEmpty()).toBe(true);
    store.shift(1);
    backend
      .expectOne((r) => r.url.endsWith('/calendar'))
      .flush({ data: { items: null, truncated: false } });
    void store.respond('e1', 'accepted');
    backend
      .expectOne((r) => r.url.endsWith('/calendar/events/e1/attendees/me'))
      .flush({ data: {} });
  });
});
