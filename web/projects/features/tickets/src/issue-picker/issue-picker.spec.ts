import { ComponentFixture, TestBed } from '@angular/core/testing';
import { guardConsole } from '@corelog/core/testing';
import { of } from 'rxjs';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { ISSUE_ID, pageOf, setupTickets, ticketOf } from '../testing/ticket-test-kit';
import { IssuePicker } from './issue-picker';

describe('IssuePicker', () => {
  const guard = guardConsole();

  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('loads the latest issues on open and never offers the current issue', () => {
    const setup = setupTickets({
      ticketApi: {
        list: () =>
          of(
            pageOf([
              ticketOf({ id: ISSUE_ID, title: 'Actual' }),
              ticketOf({ id: 'other-1', title: 'Otra incidencia' }),
            ]),
          ),
      },
    });
    const fixture: ComponentFixture<IssuePicker> = TestBed.createComponent(IssuePicker);
    fixture.componentRef.setInput('excludeId', ISSUE_ID);
    fixture.detectChanges();
    vi.advanceTimersByTime(300);
    fixture.detectChanges();

    expect(setup.ticketApi.list).toHaveBeenCalledWith({ page: 1, pageSize: 20, query: undefined });
    expect(
      fixture.nativeElement.querySelector('[aria-label="Incidencia a vincular"]'),
    ).not.toBeNull();
    guard.expectNoConsoleOutput();
  });
});
