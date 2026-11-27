import { ComponentFixture, TestBed } from '@angular/core/testing';
import { guardConsole } from '@corelog/core/testing';
import { of } from 'rxjs';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { ISSUE_ID, pageOf, setupIssues, issueOf } from '../testing/issue-test-kit';
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
    const setup = setupIssues({
      issueApi: {
        list: () =>
          of(
            pageOf([
              issueOf({ id: ISSUE_ID, summary: 'Actual' }),
              issueOf({ id: 'other-1', summary: 'Otra incidencia' }),
            ]),
          ),
      },
    });
    const fixture: ComponentFixture<IssuePicker> = TestBed.createComponent(IssuePicker);
    fixture.componentRef.setInput('excludeId', ISSUE_ID);
    fixture.detectChanges();
    vi.advanceTimersByTime(300);
    fixture.detectChanges();

    expect(setup.issueApi.list).toHaveBeenCalledWith({ page: 1, pageSize: 20, query: undefined });
    expect(
      fixture.nativeElement.querySelector('[aria-label="Incidencia a vincular"]'),
    ).not.toBeNull();
    guard.expectNoConsoleOutput();
  });

  it('shows the server results as returned, even when the title does not contain the query', () => {
    setupIssues({
      issueApi: {
        list: () => of(pageOf([issueOf({ id: 'desc-1', summary: 'Sin relación en el título' })])),
      },
    });
    const fixture: ComponentFixture<IssuePicker> = TestBed.createComponent(IssuePicker);
    fixture.componentRef.setInput('excludeId', ISSUE_ID);
    fixture.detectChanges();
    vi.advanceTimersByTime(300);

    fixture.componentInstance['search']('texto de la descripción');
    vi.advanceTimersByTime(300);
    fixture.detectChanges();

    const labels = fixture.componentInstance['options']().map((option) => option.label);
    expect(labels).toEqual(expect.arrayContaining([expect.stringContaining('Sin relación')]));
    guard.expectNoConsoleOutput();
  });
});
