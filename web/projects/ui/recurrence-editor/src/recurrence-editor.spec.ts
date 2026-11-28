import { Component, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { buildRecurrence, NO_RECURRENCE, type RecurrenceSpec } from '@corelog/core';

import { RecurrenceEditor } from './recurrence-editor';

@Component({
  imports: [RecurrenceEditor],
  template: `<app-recurrence-editor [(value)]="spec" startDate="2026-11-25" />`,
})
class Host {
  readonly spec = signal<RecurrenceSpec>(NO_RECURRENCE);
}

function setup() {
  TestBed.configureTestingModule({ imports: [Host] });
  const fixture = TestBed.createComponent(Host);
  fixture.detectChanges();
  return { fixture, host: fixture.componentInstance, el: fixture.nativeElement as HTMLElement };
}

describe('RecurrenceEditor', () => {
  it('says that the event does not repeat by default', () => {
    const { el } = setup();
    expect(el.querySelector('[data-testid="recurrence-summary"]')?.textContent).toContain(
      'No se repite',
    );
    expect(el.querySelector('[data-testid="weekday-MO"]')).toBeNull();
  });

  it('shows weekday chips for a weekly rule and keeps at least one day', () => {
    const { fixture, host, el } = setup();
    host.spec.set({ freq: 'WEEKLY', interval: 1, weekdays: ['WE'], end: { kind: 'never' } });
    fixture.detectChanges();
    const monday = el.querySelector<HTMLButtonElement>('[data-testid="weekday-MO"]');
    const wednesday = el.querySelector<HTMLButtonElement>('[data-testid="weekday-WE"]');
    expect(wednesday?.getAttribute('aria-pressed')).toBe('true');
    monday?.click();
    fixture.detectChanges();
    expect(buildRecurrence(host.spec())).toBe('FREQ=WEEKLY;BYDAY=MO,WE');
    monday?.click();
    wednesday?.click();
    fixture.detectChanges();
    expect(host.spec().weekdays).toEqual(['WE']);
  });

  it('describes the rule in Spanish and edits the repetition count', () => {
    const { fixture, host, el } = setup();
    host.spec.set({
      freq: 'WEEKLY',
      interval: 1,
      weekdays: ['MO', 'WE'],
      end: { kind: 'count', count: 10 },
    });
    fixture.detectChanges();
    expect(el.querySelector('[data-testid="recurrence-summary"]')?.textContent).toContain(
      'Cada semana los lunes y miércoles, 10 veces',
    );
    const count = el.querySelector<HTMLInputElement>('[data-testid="recurrence-count"]');
    expect(count?.disabled).toBe(false);
    if (count) {
      count.value = '4';
      count.dispatchEvent(new Event('change'));
    }
    fixture.detectChanges();
    expect(host.spec().end).toEqual({ kind: 'count', count: 4 });
  });

  it('clamps the interval', () => {
    const { fixture, host, el } = setup();
    host.spec.set({ freq: 'DAILY', interval: 1, weekdays: [], end: { kind: 'never' } });
    fixture.detectChanges();
    const input = el.querySelector<HTMLInputElement>('input[aria-label="Intervalo de repetición"]');
    if (input) {
      input.value = '500';
      input.dispatchEvent(new Event('change'));
    }
    expect(host.spec().interval).toBe(99);
  });
});
