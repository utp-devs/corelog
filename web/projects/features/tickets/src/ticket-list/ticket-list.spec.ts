import { APP_BASE_HREF } from '@angular/common';
import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { NotificationService, Ticket } from '@corelog/core';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { TicketList } from './ticket-list';

const ISSUE_ID = '9a8b7c6d-0000-4000-8000-000000000007';

const ticket = {
  id: ISSUE_ID,
  title: 'Revisar permisos',
  status: 'open',
  priority: 'medium',
  issueType: 'task',
  assigneeId: null,
  assigneeName: null,
  updatedAt: '2026-01-02T00:00:00Z',
} as unknown as Ticket;

describe('TicketList links under the workspace base href', () => {
  afterEach(() => {
    TestBed.resetTestingModule();
  });

  it('links each row key to the issue with the base prefixed once', () => {
    TestBed.configureTestingModule({
      providers: [
        provideRouter([]),
        { provide: APP_BASE_HREF, useValue: '/workspace/' },
        {
          provide: NotificationService,
          useValue: { success: vi.fn(), info: vi.fn(), error: vi.fn() },
        },
      ],
    });
    const fixture = TestBed.createComponent(TicketList);
    fixture.componentRef.setInput('tickets', [ticket]);
    fixture.detectChanges();
    const hrefs = Array.from(
      (fixture.nativeElement as HTMLElement).querySelectorAll('app-issue-key a'),
    ).map((link) => link.getAttribute('href'));
    expect(hrefs).toContain(`/workspace/issues/${ISSUE_ID}`);
    expect(hrefs.some((href) => href?.includes('/workspace/workspace/'))).toBe(false);
  });
});
