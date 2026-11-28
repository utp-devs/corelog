import { Injectable, InjectionToken, inject, signal } from '@angular/core';

import { API_URL } from '../config/api.config';
import type {
  NotificationAction,
  NotificationDto,
  SessionEvent,
} from '../models/notification.model';
import { NotificationStore } from '../state/notification.store';
import { resetOnLogout } from '../state/session-scoped';
import { sessionIdOfToken } from '../utils/session-token';
import { SseParser, type SseEvent } from '../utils/sse-parser';
import { toNotification } from './api/notification-api.service';
import { SessionService } from './session.service';

/** The `fetch` used to open the stream. Replaced in tests. */
export const STREAM_FETCH = new InjectionToken<typeof fetch>('STREAM_FETCH', {
  providedIn: 'root',
  factory: () => (input, init) => fetch(input, init),
});

/** Source of the reconnection jitter (a number in [0, 1)). Replaced in tests. */
export const STREAM_RANDOM = new InjectionToken<() => number>('STREAM_RANDOM', {
  providedIn: 'root',
  factory: () => Math.random,
});

/** Wait before reconnecting when the server did not announce one. */
export const STREAM_RETRY_MS = 3000;
/** The wait never grows beyond this, however many attempts failed. */
export const STREAM_MAX_RETRY_MS = 30_000;
/** How long a security alert stays on screen when the user does nothing. */
export const SECURITY_ALERT_LIFE_MS = 60_000;

/** A new login on the account, as the alert shows it. */
export interface SecurityAlert {
  readonly notificationId: string | null;
  /** Session that was just opened: the one "No fui yo" closes. */
  readonly sessionId: string | null;
  readonly title: string;
  readonly message: string;
  readonly actions: readonly NotificationAction[];
}

type Outcome = 'retry' | 'final';

/**
 * The live channel of the signed-in user: one long `fetch` to `/notifications/stream` per browser
 * tab (EventSource cannot send the bearer header). Notifications are pushed to the store, security
 * events raise the alert. After any close or error it reconnects with a growing, jittered wait
 * and the id of the last notification seen, so the server replays what was missed. A revoked
 * session ends the local session; a replaced stream stops without reconnecting.
 */
@Injectable({ providedIn: 'root' })
export class NotificationStream {
  private readonly session = inject(SessionService);
  private readonly store = inject(NotificationStore);
  private readonly baseUrl = inject(API_URL);
  private readonly fetchFn = inject(STREAM_FETCH);
  private readonly random = inject(STREAM_RANDOM);

  private running = false;
  private abort: AbortController | null = null;
  private waitTimer: ReturnType<typeof setTimeout> | null = null;
  private wakeUp: (() => void) | null = null;
  private alertTimer: ReturnType<typeof setTimeout> | null = null;
  private lastEventId: string | null = null;
  private retryMs = STREAM_RETRY_MS;
  private failures = 0;

  private readonly connectedState = signal(false);
  private readonly alertState = signal<SecurityAlert | null>(null);

  /** True while the stream is open. */
  readonly connected = this.connectedState.asReadonly();
  /** The new-login alert waiting for the user, if any. */
  readonly securityAlert = this.alertState.asReadonly();

  constructor() {
    resetOnLogout(() => this.stop());
  }

  /** Opens the stream. Calling it again while it runs, or without a session, has no effect. */
  start(): void {
    if (this.running || !this.session.token()) {
      return;
    }
    this.running = true;
    this.failures = 0;
    this.retryMs = STREAM_RETRY_MS;
    void this.loop();
  }

  /** Closes the stream and forgets what it carried. */
  stop(): void {
    this.running = false;
    this.abort?.abort();
    this.abort = null;
    this.clearWait();
    this.lastEventId = null;
    this.connectedState.set(false);
    this.store.setLive(false);
    this.dismissAlert();
  }

  dismissAlert(): void {
    if (this.alertTimer) {
      clearTimeout(this.alertTimer);
      this.alertTimer = null;
    }
    this.alertState.set(null);
  }

  private async loop(): Promise<void> {
    while (this.running) {
      let outcome: Outcome;
      try {
        outcome = await this.connectOnce();
      } catch {
        outcome = 'retry';
      }
      if (!this.running || outcome === 'final') {
        return;
      }
      this.connectedState.set(false);
      this.store.setLive(false);
      this.failures += 1;
      await this.wait(this.delay());
    }
  }

  /** Time to wait before the next attempt: the server pace, doubled per repeated failure, with jitter. */
  private delay(): number {
    const grown = Math.min(STREAM_MAX_RETRY_MS, this.retryMs * 2 ** Math.max(0, this.failures - 1));
    const jitter = this.failures > 1 ? this.random() * 0.2 * grown : 0;
    return Math.min(STREAM_MAX_RETRY_MS, Math.round(grown + jitter));
  }

  private wait(ms: number): Promise<void> {
    return new Promise((resolve) => {
      this.wakeUp = resolve;
      this.waitTimer = setTimeout(() => this.clearWait(), ms);
    });
  }

  private clearWait(): void {
    if (this.waitTimer) {
      clearTimeout(this.waitTimer);
      this.waitTimer = null;
    }
    this.wakeUp?.();
    this.wakeUp = null;
  }

  private async connectOnce(): Promise<Outcome> {
    const token = this.session.token();
    if (!token) {
      this.stop();
      return 'final';
    }
    const abort = new AbortController();
    this.abort = abort;
    const headers: Record<string, string> = {
      Authorization: `Bearer ${token}`,
      Accept: 'text/event-stream',
    };
    if (this.lastEventId) {
      headers['Last-Event-ID'] = this.lastEventId;
    }
    const response = await this.fetchFn(`${this.baseUrl}/notifications/stream`, {
      headers,
      cache: 'no-store',
      signal: abort.signal,
    });
    if (response.status === 401) {
      this.stop();
      this.session.expire();
      return 'final';
    }
    if (response.status === 403) {
      this.stop();
      return 'final';
    }
    if (!response.ok || !response.body) {
      return 'retry';
    }

    this.connectedState.set(true);
    this.store.setLive(true);
    this.store.refreshUnreadCount();

    const parser = new SseParser((ms) => (this.retryMs = Math.max(ms, 0)));
    const decoder = new TextDecoder();
    const reader = response.body.getReader();
    let first = true;
    for (;;) {
      const { done, value } = await reader.read();
      if (done || !this.running) {
        return 'retry';
      }
      if (first) {
        first = false;
        this.failures = 0;
      }
      for (const event of parser.push(decoder.decode(value, { stream: true }))) {
        if (this.handle(event) === 'final') {
          abort.abort();
          return 'final';
        }
      }
    }
  }

  private handle(event: SseEvent): Outcome | void {
    let payload: unknown;
    try {
      payload = JSON.parse(event.data);
    } catch {
      return;
    }
    if (event.event === 'notification') {
      const dto = payload as NotificationDto;
      this.lastEventId = event.id ?? dto.id ?? this.lastEventId;
      this.store.receive(toNotification(dto));
      return;
    }
    if (event.event === 'session') {
      return this.handleSession(payload as SessionEvent);
    }
  }

  private handleSession(event: SessionEvent): Outcome | void {
    switch (event.type) {
      case 'session_revoked':
        this.stop();
        this.session.endRevoked();
        return 'final';
      case 'stream_replaced':
        this.stop();
        return 'final';
      case 'new_login':
        this.raiseAlert(event);
        return;
    }
  }

  private raiseAlert(event: SessionEvent): void {
    const sessionId = event.data?.session_id ?? event.session_id ?? null;
    if (sessionId && sessionId === sessionIdOfToken(this.session.token())) {
      return;
    }
    this.alertState.set({
      notificationId: event.notification_id ?? null,
      sessionId,
      title: event.data?.title ?? 'Nuevo inicio de sesión',
      message: event.message ?? '',
      actions: event.data?.actions ?? [],
    });
    if (this.alertTimer) {
      clearTimeout(this.alertTimer);
    }
    this.alertTimer = setTimeout(() => this.dismissAlert(), SECURITY_ALERT_LIFE_MS);
  }
}
