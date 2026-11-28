import { Injectable, computed, inject, signal } from '@angular/core';
import { Observable, Subscription, of, tap, timer } from 'rxjs';

import { NEW_LOGIN_KIND, type AppNotification } from '../models/notification.model';
import { NotificationApiService } from '../services/api/notification-api.service';
import { SessionService } from '../services/session.service';
import { asApiError } from '../utils/api-error';
import { sessionIdOfToken } from '../utils/session-token';
import { AsyncState } from './async-state';
import { resetOnLogout } from './session-scoped';

/** Interval of the unread counter refresh. */
export const NOTIFICATION_POLL_INTERVAL_MS = 30_000;

const MESSAGE_PREFERENCE_SAVED_ON = 'Avisos en la aplicación activados';
const MESSAGE_PREFERENCE_SAVED_OFF = 'Avisos en la aplicación desactivados';

/** Ids remembered to drop a notification that arrives twice (live event and replay or list). */
const SEEN_LIMIT = 200;

/**
 * In-app notifications: the unread counter (polled), the list shown when the panel opens, the
 * read marks and the in-app preference. Polling stops when the session ends.
 * While the live stream is connected the counter is not polled: the stream feeds the list and the
 * counter, and polling resumes as a fallback when it drops.
 */
@Injectable({ providedIn: 'root' })
export class NotificationStore {
  private readonly api = inject(NotificationApiService);
  private readonly session = inject(SessionService);
  private polling: Subscription | null = null;
  private seen: string[] = [];

  private readonly liveState = signal(false);
  private readonly unreadState = signal(0);
  private readonly inAppState = signal<boolean | null>(null);
  private readonly preferenceStatusState = signal<string | null>(null);
  private readonly preferenceErrorState = signal<string | null>(null);

  readonly list = new AsyncState<AppNotification[]>();
  /** Notifications to show: the new-login alert of the current session itself is left out. */
  readonly items = computed<readonly AppNotification[]>(() =>
    (this.list.value() ?? []).filter((item) => !this.isOwnLoginAlert(item)),
  );
  /** True while the live stream is connected (polling is paused meanwhile). */
  readonly live = this.liveState.asReadonly();
  readonly unreadCount = this.unreadState.asReadonly();
  /** `null` until the preference has been read; the switch is disabled meanwhile. */
  readonly inAppEnabled = this.inAppState.asReadonly();
  readonly preferenceStatus = this.preferenceStatusState.asReadonly();
  readonly preferenceError = this.preferenceErrorState.asReadonly();

  constructor() {
    resetOnLogout(() => this.reset());
  }

  /** True while the unread counter is refreshed periodically. */
  isPolling(): boolean {
    return this.polling !== null;
  }

  /** Refreshes the unread counter now and then every 30 seconds. Calling it twice has no effect. */
  startPolling(): void {
    if (this.polling) {
      return;
    }
    this.polling = timer(0, NOTIFICATION_POLL_INTERVAL_MS).subscribe(() => {
      if (!this.liveState()) {
        this.refreshUnreadCount();
      }
    });
  }

  stopPolling(): void {
    this.polling?.unsubscribe();
    this.polling = null;
  }

  /** Reads the counter. Failures keep the previous count: a poll must not show an error. */
  refreshUnreadCount(): void {
    if (!this.session.isAuthenticated()) {
      return;
    }
    this.api.unreadCount().subscribe({
      next: (count) => this.unreadState.set(count),
      error: () => undefined,
    });
  }

  setLive(live: boolean): void {
    this.liveState.set(live);
  }

  /** True for the new-login alert that the current session itself caused. */
  isOwnLoginAlert(notification: AppNotification): boolean {
    const sessionId = notification.data?.session_id;
    return (
      notification.kind === NEW_LOGIN_KIND &&
      !!sessionId &&
      sessionId === sessionIdOfToken(this.session.token())
    );
  }

  /**
   * Adds a notification pushed by the live stream: it goes first in the list (when the list has been
   * loaded) and raises the counter if unread. One that was already received is ignored, and so is
   * the new-login alert of the current session.
   */
  receive(notification: AppNotification): void {
    if (this.isOwnLoginAlert(notification) || this.seen.includes(notification.id)) {
      return;
    }
    this.seen = [...this.seen, notification.id].slice(-SEEN_LIMIT);
    this.list.update((items) =>
      items === undefined || items.some((item) => item.id === notification.id)
        ? items
        : [notification, ...items],
    );
    if (!notification.read) {
      this.unreadState.update((count) => count + 1);
    }
  }

  loadList(): void {
    this.list.load(this.api.list());
  }

  /**
   * Marks one notification as read. Already read notifications make no request. The counter goes
   * down by one for each notification that was unread.
   */
  markRead(notification: AppNotification): Observable<void> {
    if (notification.read) {
      return of(undefined);
    }
    return this.api.markRead(notification.id).pipe(
      tap(() => {
        this.list.update((items) =>
          items?.map((item) => (item.id === notification.id ? { ...item, read: true } : item)),
        );
        this.unreadState.update((count) => Math.max(0, count - 1));
      }),
    );
  }

  markAllRead(): Observable<void> {
    return this.api.markAllRead().pipe(
      tap(() => {
        this.list.update((items) => items?.map((item) => ({ ...item, read: true })));
        this.unreadState.set(0);
      }),
    );
  }

  loadPreferences(): void {
    this.api.getPreferences().subscribe({
      next: (preferences) => this.inAppState.set(preferences.inApp),
      error: () => undefined,
    });
  }

  /**
   * Turns in-app notifications on or off. The switch changes at once; a failed save restores the
   * previous value and stores the error message.
   */
  setInAppEnabled(enabled: boolean): void {
    const previous = this.inAppState();
    this.inAppState.set(enabled);
    this.preferenceErrorState.set(null);
    this.api.updatePreferences(enabled).subscribe({
      next: (saved) => {
        this.inAppState.set(saved.inApp);
        this.preferenceStatusState.set(
          saved.inApp ? MESSAGE_PREFERENCE_SAVED_ON : MESSAGE_PREFERENCE_SAVED_OFF,
        );
      },
      error: (error: unknown) => {
        this.inAppState.set(previous);
        this.preferenceErrorState.set(asApiError(error).message);
      },
    });
  }

  reset(): void {
    this.stopPolling();
    this.liveState.set(false);
    this.seen = [];
    this.unreadState.set(0);
    this.list.reset();
    this.inAppState.set(null);
    this.preferenceStatusState.set(null);
    this.preferenceErrorState.set(null);
  }
}
