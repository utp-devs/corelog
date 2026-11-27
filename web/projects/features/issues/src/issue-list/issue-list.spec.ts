import { APP_BASE_HREF } from '@angular/common';
import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { NotificationService, Issue } from '@corelog/core';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { IssueList } from './issue-list';

const ISSUE_ID = '9a8b7c6d-0000-4000-8000-000000000007';

const issue = {
  id: ISSUE_ID,
  key: 'CORE-7',
  summary: 'Revisar permisos',
  status: 'open',
  priority: 'medium',
  issueType: 'task',
  assigneeId: null,
  assigneeName: null,
  updatedAt: '2026-01-02T00:00:00Z',
} as unknown as Issue;

function render(base: string | null = null, extra: Partial<Issue> = {}): HTMLElement {
  TestBed.configureTestingModule({
    providers: [
      provideRouter([]),
      ...(base ? [{ provide: APP_BASE_HREF, useValue: base }] : []),
      {
        provide: NotificationService,
        useValue: { success: vi.fn(), info: vi.fn(), error: vi.fn() },
      },
    ],
  });
  const fixture = TestBed.createComponent(IssueList);
  fixture.componentRef.setInput('issues', [{ ...issue, ...extra }]);
  fixture.detectChanges();
  return fixture.nativeElement as HTMLElement;
}

describe('IssueList', () => {
  afterEach(() => {
    TestBed.resetTestingModule();
  });

  it('links the title to the issue with the workspace base prefixed once', () => {
    const el = render('/workspace/');
    const hrefs = Array.from(el.querySelectorAll('a')).map((link) => link.getAttribute('href'));
    expect(hrefs).toContain('/workspace/browse/CORE-7');
    expect(hrefs.some((href) => href?.includes('/workspace/workspace/'))).toBe(false);
  });

  it('has one link per row: the key is plain text and the title is the link', () => {
    const el = render();
    const row = el.querySelector('tbody tr.data-table__row') as HTMLElement;
    expect(row.querySelectorAll('a')).toHaveLength(1);
    expect(row.querySelector('app-issue-key a')).toBeNull();
    expect(row.querySelector('a')?.textContent?.trim()).toBe('Revisar permisos');
  });

  it('keeps the row as the only tab stop', () => {
    const el = render();
    const row = el.querySelector('tbody tr.data-table__row') as HTMLElement;
    expect(row.getAttribute('tabindex')).toBe('0');
    const stops = Array.from(row.querySelectorAll<HTMLElement>('a, button')).filter(
      (node) => node.getAttribute('tabindex') !== '-1',
    );
    expect(stops).toEqual([]);
  });

  it('shows the server key and keeps it for copying', () => {
    const el = render();
    expect(el.querySelector('.issue-key__text')?.textContent?.trim()).toBe('CORE-7');
    expect(el.querySelector('.issue-key__text')?.getAttribute('title')).toBe('CORE-7');
  });

  it('mutes closed issues without striking them through', () => {
    const el = render(null, { status: 'closed' });
    const title = el.querySelector('.issue-list__title') as HTMLElement;
    expect(title.classList).toContain('issue-list__title--resolved');
    expect(el.querySelector('.issue-key__text--resolved')).not.toBeNull();
    expect(el.querySelector('[class*="line-through"]')).toBeNull();
  });

  it('shows the updated time as relative text with the exact date in a tooltip', async () => {
    const recent = new Date(Date.now() - 3 * 3_600_000).toISOString();
    const el = render(null, { updatedAt: recent });
    const time = el.querySelector('time') as HTMLElement;
    expect(time.textContent?.trim()).toBe('hace 3 horas');
    time.dispatchEvent(new Event('mouseenter'));
    await vi.waitFor(() => {
      expect(document.querySelector('[role="tooltip"]')?.textContent).toMatch(
        /^\d{2}\/\d{2}\/\d{4} \d{2}:\d{2}$/,
      );
    });
  });

  it('never wraps the status label', () => {
    const el = render();
    const lozenge = el.querySelector('app-lozenge .lozenge') as HTMLElement;
    expect(getComputedStyle(lozenge).whiteSpace).toBe('nowrap');
  });
});
