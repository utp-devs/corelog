import { provideHttpClient } from '@angular/common/http';
import { provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';
import { vi } from 'vitest';

import { provideCorelogTesting } from '../../testing';
import { LOCATION_ASSIGN } from '../config/navigation';
import { NotificationStore } from '../state/notification.store';
import {
  NotificationStream,
  SECURITY_ALERT_LIFE_MS,
  STREAM_FETCH,
  STREAM_RANDOM,
} from './notification-stream';
import { SessionService } from './session.service';

const OWN_SESSION = 'own-session';

function jwt(jti: string): string {
  return `h.${btoa(JSON.stringify({ jti }))}.s`;
}

/** A controllable response body: `push` delivers a chunk, `end` closes it. */
class FakeStream {
  private controller!: ReadableStreamDefaultController<Uint8Array>;
  readonly body = new ReadableStream<Uint8Array>({
    start: (controller) => (this.controller = controller),
  });
  push(text: string): void {
    this.controller.enqueue(new TextEncoder().encode(text));
  }
  end(): void {
    this.controller.close();
  }
}

interface Call {
  readonly headers: Record<string, string>;
  readonly stream: FakeStream;
}

function frame(id: string, extra: Record<string, unknown> = {}): string {
  const dto = {
    id,
    kind: 'issue.assigned',
    message: `Aviso ${id}`,
    link: '/x',
    read: false,
    created_at: '2026-11-28T10:00:00Z',
    ...extra,
  };
  return `id: ${id}\nevent: notification\ndata: ${JSON.stringify(dto)}\n\n`;
}

function sessionFrame(event: object): string {
  return `event: session\ndata: ${JSON.stringify(event)}\n\n`;
}

function setup(responses: (Response | Error)[] = []) {
  const calls: Call[] = [];
  const assigned: string[] = [];
  const fetchFn = vi.fn((_url: unknown, init?: RequestInit) => {
    const stream = new FakeStream();
    calls.push({ headers: init?.headers as Record<string, string>, stream });
    const next = responses.shift();
    if (next instanceof Error) return Promise.reject(next);
    return Promise.resolve(
      next ??
        new Response(stream.body, {
          status: 200,
          headers: { 'Content-Type': 'text/event-stream' },
        }),
    );
  });
  TestBed.configureTestingModule({
    providers: [
      provideCorelogTesting(),
      provideHttpClient(),
      provideHttpClientTesting(),
      { provide: STREAM_FETCH, useValue: fetchFn },
      { provide: STREAM_RANDOM, useValue: () => 0.5 },
      { provide: LOCATION_ASSIGN, useValue: (url: string) => assigned.push(url) },
    ],
  });
  const session = TestBed.inject(SessionService);
  session.startSession(jwt(OWN_SESSION));
  return {
    stream: TestBed.inject(NotificationStream),
    store: TestBed.inject(NotificationStore),
    session,
    calls,
    assigned,
    fetchFn,
  };
}

const flush = () => vi.advanceTimersByTimeAsync(0);

describe('NotificationStream', () => {
  beforeEach(() => {
    localStorage.clear();
    vi.useFakeTimers();
  });

  afterEach(() => {
    TestBed.inject(NotificationStream).stop();
    vi.useRealTimers();
  });

  it('opens one authenticated stream and feeds the store', async () => {
    const { stream, store, calls, fetchFn } = setup();
    stream.start();
    stream.start();
    await flush();
    expect(fetchFn).toHaveBeenCalledTimes(1);
    expect(fetchFn.mock.calls[0][0]).toBe('/api/v1/notifications/stream');
    expect(calls[0].headers['Authorization']).toBe(`Bearer ${jwt(OWN_SESSION)}`);
    expect(calls[0].headers['Last-Event-ID']).toBeUndefined();
    expect(stream.connected()).toBe(true);
    expect(store.live()).toBe(true);

    calls[0].stream.push('retry: 3000\n\n: ping\n\n' + frame('n1'));
    await flush();
    expect(store.unreadCount()).toBe(1);
  });

  it('prepends to a loaded list and does not count the same id twice', async () => {
    const { stream, store, calls } = setup();
    store.list.set([{ id: 'old', kind: 'k', message: 'm', link: '', read: true, createdAt: 'x' }]);
    stream.start();
    await flush();
    calls[0].stream.push(frame('n1') + frame('n1'));
    await flush();
    expect(store.items().map((n) => n.id)).toEqual(['n1', 'old']);
    expect(store.unreadCount()).toBe(1);
  });

  it('marks the store as not live when the stream closes', async () => {
    const { stream, store, calls } = setup();
    stream.start();
    await flush();
    expect(store.live()).toBe(true);
    calls[0].stream.end();
    await flush();
    expect(store.live()).toBe(false);
  });

  it('reconnects after the server pace with the last event id', async () => {
    const { stream, calls, fetchFn } = setup();
    stream.start();
    await flush();
    calls[0].stream.push('retry: 3000\n\n' + frame('n1') + frame('n2'));
    await flush();
    calls[0].stream.end();
    await flush();
    expect(fetchFn).toHaveBeenCalledTimes(1);
    await vi.advanceTimersByTimeAsync(2999);
    expect(fetchFn).toHaveBeenCalledTimes(1);
    await vi.advanceTimersByTimeAsync(1);
    expect(fetchFn).toHaveBeenCalledTimes(2);
    expect(calls[1].headers['Last-Event-ID']).toBe('n2');
  });

  it('backs off on repeated failures and resets after a good connection', async () => {
    const failure = () => new Response('x', { status: 500 });
    const { stream, calls, fetchFn } = setup([failure(), new Error('net'), failure(), failure()]);
    stream.start();
    await flush();
    // Waits: 3000, then 6000 + 10 % jitter, then 12000 + 10 %, then 24000 + 10 %.
    await vi.advanceTimersByTimeAsync(3000);
    expect(fetchFn).toHaveBeenCalledTimes(2);
    await vi.advanceTimersByTimeAsync(6599);
    expect(fetchFn).toHaveBeenCalledTimes(2);
    await vi.advanceTimersByTimeAsync(1);
    expect(fetchFn).toHaveBeenCalledTimes(3);
    await vi.advanceTimersByTimeAsync(13199);
    expect(fetchFn).toHaveBeenCalledTimes(3);
    await vi.advanceTimersByTimeAsync(1);
    expect(fetchFn).toHaveBeenCalledTimes(4);
    // The wait is capped at 30 seconds.
    await vi.advanceTimersByTimeAsync(30_000);
    expect(fetchFn).toHaveBeenCalledTimes(5);
    calls[4].stream.push(frame('n1'));
    await flush();
    calls[4].stream.end();
    await flush();
    await vi.advanceTimersByTimeAsync(3000);
    expect(fetchFn).toHaveBeenCalledTimes(6);
  });

  it('ends the local session and does not reconnect after session_revoked', async () => {
    const { stream, calls, fetchFn, session, assigned } = setup();
    stream.start();
    await flush();
    calls[0].stream.push(sessionFrame({ type: 'session_revoked' }));
    await flush();
    expect(session.isAuthenticated()).toBe(false);
    expect(assigned).toEqual(['/login?reason=session_revoked']);
    await vi.advanceTimersByTimeAsync(60_000);
    expect(fetchFn).toHaveBeenCalledTimes(1);
  });

  it('stops silently and keeps the session after stream_replaced', async () => {
    const { stream, calls, fetchFn, session, assigned } = setup();
    stream.start();
    await flush();
    calls[0].stream.push(sessionFrame({ type: 'stream_replaced' }));
    await flush();
    expect(session.isAuthenticated()).toBe(true);
    expect(assigned).toEqual([]);
    expect(stream.connected()).toBe(false);
    await vi.advanceTimersByTimeAsync(60_000);
    expect(fetchFn).toHaveBeenCalledTimes(1);
  });

  it('does not retry on 403', async () => {
    const { stream, fetchFn } = setup([new Response('no', { status: 403 })]);
    stream.start();
    await flush();
    await vi.advanceTimersByTimeAsync(60_000);
    expect(fetchFn).toHaveBeenCalledTimes(1);
  });

  it('raises the new-login alert, which expires after a minute', async () => {
    const { stream, calls } = setup();
    stream.start();
    await flush();
    calls[0].stream.push(
      sessionFrame({
        type: 'new_login',
        session_id: 'other',
        notification_id: 'n9',
        message: 'Se inició sesión desde Chrome',
        data: { title: 'Nuevo inicio de sesión', session_id: 'other', actions: [] },
      }),
    );
    await flush();
    expect(stream.securityAlert()).toEqual(
      expect.objectContaining({ sessionId: 'other', message: 'Se inició sesión desde Chrome' }),
    );
    await vi.advanceTimersByTimeAsync(SECURITY_ALERT_LIFE_MS);
    expect(stream.securityAlert()).toBeNull();
  });

  it('ignores the alert and the notification of its own session', async () => {
    const { stream, store, calls } = setup();
    stream.start();
    await flush();
    calls[0].stream.push(
      sessionFrame({
        type: 'new_login',
        session_id: OWN_SESSION,
        data: { session_id: OWN_SESSION },
      }) + frame('n3', { kind: 'security.new_login', data: { session_id: OWN_SESSION } }),
    );
    await flush();
    expect(stream.securityAlert()).toBeNull();
    expect(store.unreadCount()).toBe(0);
  });
});
