import { TestBed } from '@angular/core/testing';
import { describe, expect, it } from 'vitest';

import { sprintOf } from '../testing/agile-fixtures';
import { SprintDialog, SprintDialogMode, SprintDialogValue } from './sprint-dialog';

function render(mode: SprintDialogMode, extra: { issueCount?: number; startDate?: string } = {}) {
  TestBed.resetTestingModule();
  const fixture = TestBed.createComponent(SprintDialog);
  fixture.componentRef.setInput('open', true);
  fixture.componentRef.setInput('mode', mode);
  fixture.componentRef.setInput('suggestedName', 'Sprint 3');
  fixture.componentRef.setInput('issueCount', extra.issueCount ?? 2);
  if (mode !== 'create') {
    fixture.componentRef.setInput(
      'sprint',
      sprintOf('1', { name: 'Sprint 2', goal: 'Meta', startDate: extra.startDate ?? '2026-12-01' }),
    );
  }
  const emitted: SprintDialogValue[] = [];
  fixture.componentInstance.submitted.subscribe((value) => emitted.push(value));
  fixture.detectChanges();
  return { fixture, emitted };
}

function submit(fixture: { componentInstance: SprintDialog }): void {
  (fixture.componentInstance as unknown as { submit(): void }).submit();
}

describe('SprintDialog', () => {
  it('proposes the next name and two weeks of dates when creating', () => {
    const { fixture, emitted } = render('create');
    expect(document.body.textContent).toContain('Crear sprint');
    submit(fixture);
    expect(emitted).toHaveLength(1);
    expect(emitted[0].name).toBe('Sprint 3');
    expect(emitted[0].startDate).toMatch(/^\d{4}-\d{2}-\d{2}$/);
    expect(
      emitted[0].endDate && emitted[0].startDate && emitted[0].endDate > emitted[0].startDate,
    ).toBe(true);
    expect(emitted[0].force).toBe(false);
  });

  it('does not submit without a name', () => {
    const { fixture, emitted } = render('create');
    (
      fixture.componentInstance as unknown as {
        form: { controls: { name: { setValue(v: string): void } } };
      }
    ).form.controls.name.setValue('  ');
    submit(fixture);
    expect(emitted).toEqual([]);
  });

  it('prefills the sprint when editing', () => {
    const { fixture, emitted } = render('edit');
    expect(document.body.textContent).toContain('Editar sprint');
    submit(fixture);
    expect(emitted[0]).toMatchObject({ name: 'Sprint 2', goal: 'Meta', startDate: '2026-12-01' });
  });

  it('refuses an end date before the start', () => {
    const { fixture, emitted } = render('edit');
    const form = (
      fixture.componentInstance as unknown as {
        form: { patchValue(value: { endDate: string }): void };
      }
    ).form;
    form.patchValue({ endDate: '2026-11-01' });
    submit(fixture);
    fixture.detectChanges();
    expect(emitted).toEqual([]);
    expect(document.body.textContent).toContain('El fin no puede ser anterior al inicio');
  });

  it('asks to confirm starting a sprint with no issues', () => {
    const { fixture, emitted } = render('start', { issueCount: 0 });
    expect(document.querySelector('[data-testid="sprint-force"]')).not.toBeNull();
    submit(fixture);
    expect(emitted).toEqual([]);
    document.querySelector<HTMLInputElement>('[data-testid="sprint-force"]')?.click();
    submit(fixture);
    expect(emitted[0].force).toBe(true);
  });

  it('starts a sprint with issues without extra confirmation', () => {
    const { fixture, emitted } = render('start', { issueCount: 3 });
    expect(document.querySelector('[data-testid="sprint-force"]')).toBeNull();
    submit(fixture);
    expect(emitted[0]).toMatchObject({ name: 'Sprint 2', force: false });
  });
});
