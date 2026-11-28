import { HttpClient } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable, map } from 'rxjs';

import { API_URL } from '../../config/api.config';
import { normalizeIssueLink } from '../../config/issue-paths';
import type {
  AppNotification,
  EmailNotificationMode,
  NotificationPreferences,
  NotificationPreferencesChange,
} from '../../models/notification.model';
import { backgroundRequest, silentRequest } from '../../utils/silent-request';

interface NotificationDto {
  readonly id: string;
  readonly kind: string;
  readonly message: string;
  readonly link: string;
  readonly read: boolean;
  readonly created_at: string;
}

interface PreferencesDto {
  readonly in_app: boolean;
  readonly email?: EmailNotificationMode;
  readonly notify_own_actions?: boolean;
}

interface DataEnvelope<T> {
  readonly data: T;
}

function toNotification(dto: NotificationDto): AppNotification {
  return {
    id: dto.id,
    kind: dto.kind,
    message: dto.message,
    link: normalizeIssueLink(dto.link),
    read: dto.read,
    createdAt: dto.created_at,
  };
}

function toPreferences(dto: PreferencesDto): NotificationPreferences {
  return {
    inApp: dto.in_app,
    email: dto.email ?? 'immediate',
    notifyOwnActions: dto.notify_own_actions ?? false,
  };
}

/** In-app notifications of the signed-in user and their preferences. */
@Injectable({ providedIn: 'root' })
export class NotificationApiService {
  private readonly http = inject(HttpClient);
  private readonly baseUrl = inject(API_URL);

  list(): Observable<AppNotification[]> {
    return this.http
      .get<DataEnvelope<NotificationDto[]>>(`${this.baseUrl}/notifications`, {
        context: silentRequest(),
      })
      .pipe(map((response) => response.data.map(toNotification)));
  }

  /** Polled every 30 seconds: failures are silent and the loading bar stays off. */
  unreadCount(): Observable<number> {
    return this.http
      .get<DataEnvelope<{ readonly count: number }>>(`${this.baseUrl}/notifications/unread-count`, {
        context: backgroundRequest(),
      })
      .pipe(map((response) => response.data.count));
  }

  markRead(id: string): Observable<void> {
    return this.http.post<void>(`${this.baseUrl}/notifications/${id}/read`, null, {
      context: silentRequest(),
    });
  }

  markAllRead(): Observable<void> {
    return this.http.post<void>(`${this.baseUrl}/notifications/read-all`, null, {
      context: silentRequest(),
    });
  }

  getPreferences(): Observable<NotificationPreferences> {
    return this.http
      .get<DataEnvelope<PreferencesDto>>(`${this.baseUrl}/notifications/preferences`, {
        context: silentRequest(),
      })
      .pipe(map((response) => toPreferences(response.data)));
  }

  updatePreferences(
    inApp: boolean,
    change: NotificationPreferencesChange = {},
  ): Observable<NotificationPreferences> {
    return this.http
      .put<DataEnvelope<PreferencesDto>>(
        `${this.baseUrl}/notifications/preferences`,
        {
          in_app: inApp,
          ...(change.email !== undefined ? { email: change.email } : {}),
          ...(change.notifyOwnActions !== undefined
            ? { notify_own_actions: change.notifyOwnActions }
            : {}),
        },
        { context: silentRequest() },
      )
      .pipe(map((response) => toPreferences(response.data)));
  }
}
