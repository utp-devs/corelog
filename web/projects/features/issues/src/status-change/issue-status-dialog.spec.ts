import { signal } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { Issue, IssueStore, IssueTransition, StatusCatalogStore } from '@corelog/core';
import { provideCorelogTesting } from '@corelog/core/testing';
import { of } from 'rxjs';
import { afterEach, describe, expect, it, vi } from 'vitest';

import { IssueStatusDialog } from './issue-status-dialog';

function issue(id: string, statusId: string): Issue {
  return {
    id,
    key: `CORE-${id}`,
    summary: `Incidencia ${id}`,
    status: 'open',
    workflowStatus: { id: statusId, name: statusId, category: 'todo' },
    assigneeId: null,
  } as unknown as Issue;
}

const START: IssueTransition = {
  id: 'start',
  name: 'Iniciar',
  toStatus: 'in_progress',
  to: { id: 's2', name: 'En curso', category: 'in_progress' },
  requiredFields: [],
  requiresComment: false,
  requiresResolution: false,
  requiresAssignee: false,
  needsScreen: false,
};

function mount(issues: Issue[], fakeStore: Partial<Record<string, unknown>>) {
  TestBed.configureTestingModule({
    providers: [
      provideCorelogTesting(),
      { provide: IssueStore, useValue: fakeStore },
      {
        provide: StatusCatalogStore,
        useValue: {
          statuses: signal([{ id: 'x1', name: 'Hecho', category: 'done', position: 0 }]),
          ensureLoaded: () => undefined,
        },
      },
    ],
  });
  const fixture: ComponentFixture<IssueStatusDialog> = TestBed.createComponent(IssueStatusDialog);
  fixture.componentRef.setInput('issues', issues);
  fixture.componentRef.setInput('open', true);
  fixture.detectChanges();
  fixture.detectChanges();
  return fixture;
}

function confirm(fixture: ComponentFixture<IssueStatusDialog>): void {
  (
    document.querySelector('[data-testid="status-change-confirm"] button') as HTMLButtonElement
  ).click();
  fixture.detectChanges();
}

describe('IssueStatusDialog', () => {
  afterEach(() => TestBed.resetTestingModule());

  it('lists the transitions of one issue', () => {
    const listTransitions = vi.fn(() => of([START]));
    mount([issue('1', 's1')], { listTransitions });
    expect(listTransitions).toHaveBeenCalledWith('1');
    expect(document.querySelector('[data-testid="status-choice-start"]')).not.toBeNull();
    expect(document.body.textContent).toContain('En curso');
  });

  it('runs one transition for several issues that share a status, by transition id', () => {
    const bulkTransition = vi.fn(() => of({ succeeded: [issue('1', 's2')], failed: [] }));
    const fixture = mount([issue('1', 's1'), issue('2', 's1')], {
      listTransitions: () => of([START]),
      bulkTransition,
    });
    (document.querySelector('[data-testid="status-choice-start"]') as HTMLInputElement).click();
    fixture.detectChanges();
    confirm(fixture);
    expect(bulkTransition).toHaveBeenCalledWith(['1', '2'], { transitionId: 'start' });
  });

  it('offers the organization statuses when the issues are in different statuses', () => {
    const bulkTransition = vi.fn(() =>
      of({
        succeeded: [issue('1', 'x1')],
        failed: [
          { issue: 'CORE-2', code: 'issue.transition_unavailable', message: 'No hay transición' },
        ],
      }),
    );
    const fixture = mount([issue('1', 's1'), issue('2', 's3')], { bulkTransition });
    (document.querySelector('[data-testid="status-choice-x1"]') as HTMLInputElement).click();
    fixture.detectChanges();
    confirm(fixture);
    expect(bulkTransition).toHaveBeenCalledWith(['1', '2'], { statusId: 'x1' });
    // The result lists what failed, per issue.
    expect(document.querySelector('[data-testid="status-bulk-result"]')?.textContent).toContain(
      'CORE-2: No hay transición',
    );
  });
});
