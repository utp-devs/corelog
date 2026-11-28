import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';

import { provideCorelogTesting } from '../../testing';
import type { AppNotification } from '../models/notification.model';
import { SessionService } from '../services/session.service';
import { NOTIFICATION_POLL_INTERVAL_MS, NotificationStore } from './notification.store';

const COUNT_URL = '/api/v1/notifications/unread-count';
const PREFERENCES_URL = '/api/v1/notifications/preferences';

function notification(id: string, read: boolean): AppNotification {
  return {
    id,
    kind: 'issue_assigned',
    message: `Aviso ${id}`,
    link: `/issues/${id}`,
    read,
    createdAt: '2026-11-22T10:00:00Z',
  };
}

function setup(): {
  store: NotificationStore;
  backend: HttpTestingController;
  session: SessionService;
} {
  TestBed.configureTestingModule({
    providers: [provideCorelogTesting(), provideHttpClient(), provideHttpClientTesting()],
  });
  return {
    store: TestBed.inject(NotificationStore),
    backend: TestBed.inject(HttpTestingController),
    session: TestBed.inject(SessionService),
  };
}

/** Signs in and starts polling. The first tick fires at once, so the first request is flushed here. */
function startSignedIn(count = 3): ReturnType<typeof setup> {
  const context = setup();
  context.session.startSession('token-1');
  context.store.startPolling();
  vi.advanceTimersByTime(0);
  context.backend.expectOne(COUNT_URL).flush({ data: { count } });
  return context;
}

describe('NotificationStore', () => {
  beforeEach(() => {
    localStorage.clear();
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('polls the unread counter at 30 second intervals', () => {
    const { store, backend } = startSignedIn(3);
    expect(store.unreadCount()).toBe(3);
    expect(NOTIFICATION_POLL_INTERVAL_MS).toBe(30_000);

    vi.advanceTimersByTime(29_999);
    backend.expectNone(COUNT_URL);

    vi.advanceTimersByTime(1);
    backend.expectOne(COUNT_URL).flush({ data: { count: 5 } });
    expect(store.unreadCount()).toBe(5);

    vi.advanceTimersByTime(NOTIFICATION_POLL_INTERVAL_MS);
    backend.expectOne(COUNT_URL).flush({ data: { count: 0 } });
    expect(store.unreadCount()).toBe(0);
    backend.verify();
  });

  it('does not start a second timer when polling is already running', () => {
    const { store, backend } = startSignedIn();

    store.startPolling();
    expect(store.isPolling()).toBe(true);

    vi.advanceTimersByTime(NOTIFICATION_POLL_INTERVAL_MS);
    backend.expectOne(COUNT_URL).flush({ data: { count: 4 } });
    backend.verify();
  });

  it('stops polling when asked to', () => {
    const { store, backend } = startSignedIn();

    store.stopPolling();
    vi.advanceTimersByTime(NOTIFICATION_POLL_INTERVAL_MS * 4);

    expect(store.isPolling()).toBe(false);
    backend.verify();
  });

  it('keeps the previous count when a poll fails', () => {
    const { store, backend } = startSignedIn(3);

    vi.advanceTimersByTime(NOTIFICATION_POLL_INTERVAL_MS);
    backend.expectOne(COUNT_URL).flush({}, { status: 500, statusText: 'Server Error' });

    expect(store.unreadCount()).toBe(3);
    backend.verify();
  });

  it('pauses the polling while signed out and stops it when the session ends', () => {
    const { store, backend, session } = startSignedIn(2);

    session.clear();
    TestBed.flushEffects();
    vi.advanceTimersByTime(NOTIFICATION_POLL_INTERVAL_MS * 2);

    expect(store.isPolling()).toBe(false);
    backend.verify();
  });

  it('loads the list and marks one notification as read, lowering the counter', () => {
    const { store, backend } = startSignedIn(2);
    store.loadList();
    backend.expectOne('/api/v1/notifications').flush({
      data: [
        {
          id: 'n-1',
          kind: 'issue_assigned',
          message: 'Uno',
          link: '/issues/1',
          read: false,
          created_at: '2026-11-22T10:00:00Z',
        },
        {
          id: 'n-2',
          kind: 'issue_assigned',
          message: 'Dos',
          link: '/issues/2',
          read: true,
          created_at: '2026-11-22T09:00:00Z',
        },
      ],
    });
    expect(store.items().map((item) => item.read)).toEqual([false, true]);

    let done = false;
    store.markRead(notification('n-1', false)).subscribe(() => (done = true));
    backend
      .expectOne('/api/v1/notifications/n-1/read')
      .flush(null, { status: 204, statusText: 'No Content' });

    expect(done).toBe(true);
    expect(store.items()[0].read).toBe(true);
    expect(store.unreadCount()).toBe(1);
    backend.verify();
  });

  it('does not call the server or change the counter for a notification already read', () => {
    const { store, backend } = startSignedIn(2);

    store.markRead(notification('n-2', true)).subscribe();

    backend.verify();
    expect(store.unreadCount()).toBe(2);
  });

  it('marks every notification as read and resets the counter', () => {
    const { store, backend } = startSignedIn(4);
    store.loadList();
    backend.expectOne('/api/v1/notifications').flush({
      data: [
        {
          id: 'n-1',
          kind: 'k',
          message: 'Uno',
          link: '/',
          read: false,
          created_at: '2026-11-22T10:00:00Z',
        },
      ],
    });

    store.markAllRead().subscribe();
    backend
      .expectOne('/api/v1/notifications/read-all')
      .flush(null, { status: 204, statusText: 'No Content' });

    expect(store.unreadCount()).toBe(0);
    expect(store.items().every((item) => item.read)).toBe(true);
    backend.verify();
  });

  it('turns the in-app preference off at once and keeps it when the server accepts it', () => {
    const { store, backend } = startSignedIn();
    store.loadPreferences();
    backend.expectOne(PREFERENCES_URL).flush({ data: { in_app: true } });
    expect(store.inAppEnabled()).toBe(true);

    store.setInAppEnabled(false);
    expect(store.inAppEnabled()).toBe(false);
    const put = backend.expectOne(PREFERENCES_URL);
    expect(put.request.body).toEqual({ in_app: false });
    put.flush({ data: { in_app: false } });

    expect(store.inAppEnabled()).toBe(false);
    expect(store.preferenceStatus()).toBe('Avisos en la aplicación desactivados');
    expect(store.preferenceError()).toBeNull();
    backend.verify();
  });

  it('rolls the preference back and stores the error when the save fails', () => {
    const { store, backend } = startSignedIn();
    store.loadPreferences();
    backend.expectOne(PREFERENCES_URL).flush({ data: { in_app: true } });

    store.setInAppEnabled(false);
    backend
      .expectOne(PREFERENCES_URL)
      .flush(
        { error: { message: 'No se pudo guardar tu preferencia.', code: 'preferences.save' } },
        { status: 500, statusText: 'Server Error' },
      );

    expect(store.inAppEnabled()).toBe(true);
    expect(store.preferenceError()).toBe('No se pudo guardar tu preferencia.');
    expect(store.preferenceStatus()).toBeNull();
    backend.verify();
  });

  it('keeps the preference unknown while it has not been read', () => {
    const { store } = startSignedIn();
    expect(store.inAppEnabled()).toBeNull();
  });

  it('clears the counter, list and preferences when the session ends', () => {
    const { store, backend, session } = startSignedIn(5);
    store.loadList();
    backend.expectOne('/api/v1/notifications').flush({ data: [] });

    session.clear();
    TestBed.flushEffects();

    expect(store.unreadCount()).toBe(0);
    expect(store.list.hasValue()).toBe(false);
    expect(store.inAppEnabled()).toBeNull();
    backend.verify();
  });
});

describe('NotificationStore live updates', () => {
  beforeEach(() => {
    localStorage.clear();
  });

  function item(id: string, extra: Partial<AppNotification> = {}): AppNotification {
    return { ...notification(id, false), ...extra };
  }

  it('hides the new-login alert of the current session from the list', () => {
    const { store, session } = setup();
    session.startSession(`h.${btoa(JSON.stringify({ jti: 's-own' }))}.s`);
    store.list.set([
      item('a', { kind: 'security.new_login', data: { session_id: 's-own' } }),
      item('b', { kind: 'security.new_login', data: { session_id: 's-other' } }),
      item('c'),
    ]);
    expect(store.items().map((n) => n.id)).toEqual(['b', 'c']);
  });

  it('counts a pushed notification once and ignores read ones in the badge', () => {
    const { store } = setup();
    store.receive(item('a'));
    store.receive(item('a'));
    store.receive(item('b', { read: true }));
    expect(store.unreadCount()).toBe(1);
  });
});
