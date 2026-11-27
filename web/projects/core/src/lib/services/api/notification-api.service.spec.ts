import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';

import { provideCorelogTesting } from '../../../testing';
import type { AppNotification } from '../../models/notification.model';
import { NotificationApiService } from './notification-api.service';

function setup(): { service: NotificationApiService; backend: HttpTestingController } {
  TestBed.configureTestingModule({
    providers: [provideCorelogTesting(), provideHttpClient(), provideHttpClientTesting()],
  });
  return {
    service: TestBed.inject(NotificationApiService),
    backend: TestBed.inject(HttpTestingController),
  };
}

describe('NotificationApiService', () => {
  it('maps notifications from snake_case', () => {
    const { service, backend } = setup();
    let items: AppNotification[] = [];

    service.list().subscribe((value) => (items = value));
    backend.expectOne('/api/v1/notifications').flush({
      data: [
        {
          id: 'n-1',
          kind: 'issue_assigned',
          message: 'Te asignaron',
          link: '/issues/1',
          read: false,
          created_at: '2026-11-22T10:00:00Z',
        },
      ],
    });

    expect(items[0]).toEqual({
      id: 'n-1',
      kind: 'issue_assigned',
      message: 'Te asignaron',
      link: '/browse/1',
      read: false,
      createdAt: '2026-11-22T10:00:00Z',
    });
    backend.verify();
  });

  it('reads the unread counter and marks notifications as read', () => {
    const { service, backend } = setup();
    let count: number | undefined;

    service.unreadCount().subscribe((value) => (count = value));
    backend.expectOne('/api/v1/notifications/unread-count').flush({ data: { count: 3 } });
    expect(count).toBe(3);

    service.markRead('n-1').subscribe();
    const read = backend.expectOne('/api/v1/notifications/n-1/read');
    expect(read.request.method).toBe('POST');
    read.flush(null, { status: 204, statusText: 'No Content' });

    service.markAllRead().subscribe();
    const all = backend.expectOne('/api/v1/notifications/read-all');
    expect(all.request.method).toBe('POST');
    all.flush(null, { status: 204, statusText: 'No Content' });
    backend.verify();
  });

  it('reads and writes the in-app preference', () => {
    const { service, backend } = setup();
    let prefs: { inApp: boolean } | undefined;

    service.getPreferences().subscribe((value) => (prefs = value));
    backend.expectOne('/api/v1/notifications/preferences').flush({ data: { in_app: true } });
    expect(prefs).toEqual({ inApp: true });

    service.updatePreferences(false).subscribe((value) => (prefs = value));
    const put = backend.expectOne('/api/v1/notifications/preferences');
    expect(put.request.method).toBe('PUT');
    expect(put.request.body).toEqual({ in_app: false });
    put.flush({ data: { in_app: false } });
    expect(prefs).toEqual({ inApp: false });
    backend.verify();
  });
});
