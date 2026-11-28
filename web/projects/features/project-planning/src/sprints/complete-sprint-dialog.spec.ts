import { TestBed } from '@angular/core/testing';
import { CompleteSprintInput } from '@corelog/core';
import { describe, expect, it } from 'vitest';

import { sprintOf } from '../testing/agile-fixtures';
import { CompleteSprintDialog, CompleteSprintSummary } from './complete-sprint-dialog';

const SUMMARY: CompleteSprintSummary = {
  doneIssues: 2,
  openIssues: 3,
  donePoints: 5,
  openPoints: 8,
};

function render(summary: CompleteSprintSummary = SUMMARY, planned = [sprintOf('2')]) {
  TestBed.resetTestingModule();
  const fixture = TestBed.createComponent(CompleteSprintDialog);
  fixture.componentRef.setInput('open', true);
  fixture.componentRef.setInput('sprint', sprintOf('1', { status: 'active' }));
  fixture.componentRef.setInput('summary', summary);
  fixture.componentRef.setInput('plannedSprints', planned);
  const emitted: CompleteSprintInput[] = [];
  fixture.componentInstance.confirmed.subscribe((value) => emitted.push(value));
  fixture.detectChanges();
  return { fixture, emitted };
}

function click(testId: string): void {
  document.querySelector<HTMLElement>(`[data-testid="${testId}"]`)?.click();
}

describe('CompleteSprintDialog', () => {
  it('summarises what is finished and what is not', () => {
    render();
    expect(document.querySelector('[data-testid="complete-done"]')?.textContent).toContain(
      '2 incidencias terminadas (5 pts)',
    );
    expect(document.querySelector('[data-testid="complete-open"]')?.textContent).toContain(
      '3 incidencias sin terminar (8 pts)',
    );
  });

  it('sends the unfinished issues back to the backlog by default', () => {
    const { fixture, emitted } = render();
    fixture.componentInstance['submit']();
    expect(emitted).toEqual([{ destination: 'backlog' }]);
  });

  it('offers a planned sprint as destination', () => {
    const { fixture, emitted } = render();
    click('destination-sprint');
    fixture.detectChanges();
    fixture.componentInstance['submit']();
    expect(emitted).toEqual([{ destination: 'sprint:2' }]);
  });

  it('creates a new sprint, with the typed name when there is one', () => {
    const { fixture, emitted } = render();
    click('destination-new');
    fixture.detectChanges();
    fixture.componentInstance['newName'].set('Sprint 14');
    fixture.componentInstance['submit']();
    fixture.componentInstance['newName'].set('   ');
    fixture.componentInstance['submit']();
    expect(emitted).toEqual([
      { destination: 'new', newSprintName: 'Sprint 14' },
      { destination: 'new', newSprintName: undefined },
    ]);
  });

  it('disables the planned-sprint option when none exists', () => {
    render(SUMMARY, []);
    expect(
      document.querySelector<HTMLInputElement>('[data-testid="destination-sprint"]')?.disabled,
    ).toBe(true);
  });

  it('skips the question when everything is done', () => {
    const { fixture, emitted } = render({
      doneIssues: 4,
      openIssues: 0,
      donePoints: 9,
      openPoints: 0,
    });
    expect(document.querySelector('[data-testid="destination-backlog"]')).toBeNull();
    fixture.componentInstance['submit']();
    expect(emitted).toEqual([{ destination: 'backlog' }]);
  });
});
