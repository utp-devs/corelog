import { ComponentFixture, TestBed } from '@angular/core/testing';
import type { ReportQuery } from '@corelog/core';
import { describe, expect, it } from 'vitest';

import { ReportParamsBar } from './report-params-bar';

const TODAY = '2026-11-27';

function create(
  kind: string,
  params: string[],
  query: ReportQuery = {},
): { fixture: ComponentFixture<ReportParamsBar>; emitted: ReportQuery[] } {
  const fixture = TestBed.createComponent(ReportParamsBar);
  fixture.componentRef.setInput('kind', kind);
  fixture.componentRef.setInput('params', params);
  fixture.componentRef.setInput('query', query);
  fixture.componentRef.setInput('today', TODAY);
  const emitted: ReportQuery[] = [];
  fixture.componentInstance.queryChange.subscribe((value) => emitted.push(value));
  fixture.detectChanges();
  return { fixture, emitted };
}

function el(fixture: ComponentFixture<ReportParamsBar>): HTMLElement {
  return fixture.nativeElement as HTMLElement;
}

describe('ReportParamsBar', () => {
  it('shows only the controls the report accepts', () => {
    const { fixture } = create('workload', ['filter']);
    expect(el(fixture).querySelector('[data-testid="report-preset-7"]')).toBeNull();
    expect(el(fixture).querySelector('app-coreql-input')).not.toBeNull();

    const range = create('control', ['from', 'to', 'filter']).fixture;
    expect(el(range).querySelectorAll('[data-testid^="report-preset-"]')).toHaveLength(3);
    expect(el(range).querySelectorAll('app-date-field')).toHaveLength(2);
  });

  it('applies a range preset ending today', () => {
    const { fixture, emitted } = create('control', ['from', 'to']);
    el(fixture).querySelector<HTMLButtonElement>('[data-testid="report-preset-7"]')?.click();
    expect(emitted).toEqual([{ from: '2026-11-21', to: TODAY }]);
  });

  it('marks the preset that matches the range, 30 days by default', () => {
    const { fixture } = create('control', ['from', 'to']);
    const pressed = Array.from(el(fixture).querySelectorAll('[data-testid^="report-preset-"]')).map(
      (button) => button.getAttribute('aria-pressed'),
    );
    expect(pressed).toEqual(['false', 'true', 'false']);
  });

  it('keeps the other parameters when one changes', () => {
    const { fixture, emitted } = create('control', ['from', 'to', 'filter'], {
      filter: 'type = bug',
    });
    el(fixture).querySelector<HTMLButtonElement>('[data-testid="report-preset-90"]')?.click();
    expect(emitted[0]).toEqual({ filter: 'type = bug', from: '2026-08-30', to: TODAY });
  });

  it('applies the filter only with its button and when it changed', () => {
    const { fixture, emitted } = create('workload', ['filter']);
    const apply = el(fixture).querySelector<HTMLButtonElement>(
      '[data-testid="report-apply-filter"]',
    );
    expect(apply?.disabled).toBe(true);

    const area = el(fixture).querySelector<HTMLTextAreaElement>('textarea');
    area!.value = ' priority = high ';
    area!.dispatchEvent(new Event('input'));
    fixture.detectChanges();
    expect(apply?.disabled).toBe(false);
    apply?.click();
    expect(emitted).toEqual([{ filter: 'priority = high' }]);
  });

  it('removes the filter when it is cleared', () => {
    const { fixture, emitted } = create('workload', ['filter'], { filter: 'x = 1' });
    const area = el(fixture).querySelector<HTMLTextAreaElement>('textarea');
    expect(area?.value).toBe('x = 1');
    area!.value = '';
    area!.dispatchEvent(new Event('input'));
    fixture.detectChanges();
    el(fixture).querySelector<HTMLButtonElement>('[data-testid="report-apply-filter"]')?.click();
    expect(emitted).toEqual([{}]);
  });

  it('offers the grouping of the report only', async () => {
    const { fixture } = create('resolution-time', ['from', 'to', 'group_by']);
    await fixture.whenStable();
    fixture.detectChanges();
    expect(el(fixture).textContent).toContain('Agrupar por');
    expect(el(fixture).querySelectorAll('app-select-field')).toHaveLength(1);
  });
});
