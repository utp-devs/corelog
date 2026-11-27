import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';

import { provideCorelogTesting } from '../../testing';
import { SessionService } from '../services/session.service';
import { isoDate, startOfMonth } from '../utils/calendar-dates';
import { CalendarStore } from './calendar.store';

const EVENTS_PATH = '/api/v1/calendar/events?project_id=project-1';

function event(date: string, sourceId: string, title = sourceId) {
  return { kind: 'issue_due', date, title, source_id: sourceId, link: `/issues/${sourceId}` };
}

function setup(): {
  store: CalendarStore;
  backend: HttpTestingController;
  session: SessionService;
} {
  TestBed.configureTestingModule({
    providers: [provideCorelogTesting(), provideHttpClient(), provideHttpClientTesting()],
  });
  return {
    store: TestBed.inject(CalendarStore),
    backend: TestBed.inject(HttpTestingController),
    session: TestBed.inject(SessionService),
  };
}

/** Expected URL for the month shown by the store, as the store computes it. */
function urlFor(store: CalendarStore): string {
  const { from, to } = store.range();
  return `${EVENTS_PATH}&from=${from}&to=${to}`;
}

describe('CalendarStore', () => {
  beforeEach(() => localStorage.clear());

  it('requests the visible month and groups the events by day', () => {
    const { store, backend } = setup();
    store.load('project-1');
    const range = store.range();
    expect(range.from.endsWith('-01')).toBe(true);
    expect(range.from).toBe(isoDate(startOfMonth(store.month())));

    backend.expectOne(urlFor(store)).flush({
      data: [
        event(range.from, 'issue-1'),
        event(range.from, 'issue-2'),
        event(range.to, 'issue-3'),
      ],
    });

    expect(store.eventsByDate()[range.from].map((entry) => entry.sourceId)).toEqual([
      'issue-1',
      'issue-2',
    ]);
    expect(store.days().map((day) => day.date)).toEqual([range.from, range.to]);
    expect(store.isEmpty()).toBe(false);
    backend.verify();
  });

  it('shows the next month and cancels the request still in flight', () => {
    const { store, backend } = setup();
    store.load('project-1');
    const first = backend.expectOne(urlFor(store));
    const before = store.month();

    store.shift(1);

    expect(store.month().getMonth()).toBe((before.getMonth() + 1) % 12);
    expect(first.cancelled).toBe(true);
    backend.expectOne(urlFor(store)).flush({ data: [] });
    expect(store.isEmpty()).toBe(true);
    backend.verify();
  });

  it('keeps the error of a failed request and retries the same month', () => {
    const { store, backend } = setup();
    store.load('project-1');
    backend.expectOne(urlFor(store)).flush({}, { status: 403, statusText: 'Forbidden' });

    expect(store.events.error()?.status).toBe(403);
    store.retry();
    backend.expectOne(urlFor(store)).flush({ data: [] });
    expect(store.events.error()).toBeNull();
    backend.verify();
  });

  it('makes no request before a project is loaded', () => {
    const { store, backend } = setup();
    store.shift(1);
    store.retry();
    backend.verify();
  });

  it('clears the month and events when the session ends', () => {
    const { store, backend, session } = setup();
    store.load('project-1');
    backend.expectOne(urlFor(store)).flush({ data: [event('2026-11-05', 'issue-1')] });
    store.shift(2);
    backend.expectOne(urlFor(store)).flush({ data: [] });

    session.startSession('token');
    session.clear();
    TestBed.flushEffects();

    expect(store.events.hasValue()).toBe(false);
    expect(store.month()).toEqual(startOfMonth(new Date()));
  });
});
