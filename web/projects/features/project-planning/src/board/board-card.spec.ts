import { APP_BASE_HREF } from '@angular/common';
import { TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { BoardCard, NotificationService } from '@corelog/core';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { BoardCardView } from './board-card';

const ISSUE_ID = '5f1c2d3e-0000-4000-8000-000000000042';

const card: BoardCard = {
  id: ISSUE_ID,
  type: 'task',
  summary: 'Exportar el tablero',
  status: 'open',
  priority: 'medium',
  assignee: 'Ana',
  assigneeId: 'u-1',
  storyPoints: 3,
  rank: '0|a',
};

function render(inputs: Record<string, unknown> = {}): HTMLElement {
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
  const fixture = TestBed.createComponent(BoardCardView);
  fixture.componentRef.setInput('card', card);
  for (const [name, value] of Object.entries(inputs)) fixture.componentRef.setInput(name, value);
  fixture.detectChanges();
  return fixture.nativeElement as HTMLElement;
}

describe('BoardCardView links under the workspace base href', () => {
  afterEach(() => {
    TestBed.resetTestingModule();
  });

  it('links the key to the issue with the base prefixed once', () => {
    const el = render();
    const keyLink = el.querySelector('app-issue-key a');
    expect(keyLink?.getAttribute('href')).toBe(`/workspace/issues/${ISSUE_ID}`);
  });

  it('links the summary to the issue with the base prefixed once', () => {
    const el = render();
    const summary = el.querySelector<HTMLAnchorElement>('a.board-card__summary');
    expect(summary?.textContent?.trim()).toBe('Exportar el tablero');
    expect(summary?.getAttribute('href')).toBe(`/workspace/issues/${ISSUE_ID}`);
  });

  it('offers the move menu only to editors and emits the chosen column', () => {
    const targets = [{ id: 'c2', name: 'En curso', disabled: false }];
    const el = render({ targets });
    expect(el.querySelector('.board-card__menu-trigger')).not.toBeNull();

    TestBed.resetTestingModule();
    const readOnly = render({ targets, readOnly: true });
    expect(readOnly.querySelector('.board-card__menu-trigger')).toBeNull();
  });
});
