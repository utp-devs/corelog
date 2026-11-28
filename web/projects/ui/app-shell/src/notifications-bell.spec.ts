import { signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { NotificationStore, NotificationStream, SecurityAlertActions } from '@corelog/core';
import { of } from 'rxjs';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { NotificationsBell } from './notifications-bell';

const streamStart = vi.fn();

function setup(unread: number) {
  const store = {
    unreadCount: signal(unread),
    items: signal([]),
    startPolling: vi.fn(),
    stopPolling: vi.fn(),
    loadList: vi.fn(),
    markRead: vi.fn(() => of(undefined)),
    markAllRead: vi.fn(() => of(undefined)),
  };
  TestBed.configureTestingModule({
    imports: [NotificationsBell],
    providers: [
      provideRouter([]),
      { provide: NotificationStore, useValue: store },
      { provide: NotificationStream, useValue: { start: streamStart } },
      { provide: SecurityAlertActions, useValue: {} },
    ],
  });
  const fixture = TestBed.createComponent(NotificationsBell);
  fixture.detectChanges();
  return { fixture, store };
}

describe('NotificationsBell', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    TestBed.resetTestingModule();
  });

  it('shows the unread count in the bell label', () => {
    const { fixture, store } = setup(2);
    const bell = fixture.nativeElement.querySelector(
      'button[aria-label="Notificaciones, 2 sin leer"]',
    );
    expect(bell).not.toBeNull();
    expect(store.startPolling).toHaveBeenCalled();
    expect(streamStart).toHaveBeenCalled();
  });

  it('keeps the plain label when nothing is unread', () => {
    const { fixture } = setup(0);
    expect(
      fixture.nativeElement.querySelector('button[aria-label="Notificaciones"]'),
    ).not.toBeNull();
  });
});
