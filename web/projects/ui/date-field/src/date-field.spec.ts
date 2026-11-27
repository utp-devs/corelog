import { Component, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { FormControl, ReactiveFormsModule } from '@angular/forms';

import { DateField } from './date-field';
import { parseIsoDate, toIsoDate } from './date-value';

@Component({
  imports: [DateField, ReactiveFormsModule],
  template: `<app-date-field
    inputId="due"
    ariaLabel="Fecha de inicio"
    [formControl]="control"
    [min]="min()"
    [max]="max()"
    [invalid]="invalid()"
  />`,
})
class DateHost {
  readonly control = new FormControl<string | null>('2026-12-01');
  readonly min = signal<string | null>('2026-11-01');
  readonly max = signal<string | null>('2026-12-31');
  readonly invalid = signal(false);
}

function setup() {
  TestBed.configureTestingModule({ imports: [DateHost] });
  const fixture = TestBed.createComponent(DateHost);
  fixture.detectChanges();
  return { fixture, host: fixture.componentInstance, el: fixture.nativeElement as HTMLElement };
}

describe('date helpers', () => {
  it('parses ISO dates as local days and rejects impossible ones', () => {
    const date = parseIsoDate('2026-02-03');
    expect([date?.getFullYear(), date?.getMonth(), date?.getDate()]).toEqual([2026, 1, 3]);
    expect(parseIsoDate('2026-02-31')).toBeNull();
    expect(parseIsoDate('')).toBeNull();
    expect(parseIsoDate(null)).toBeNull();
  });

  it('formats a date back to the same ISO day', () => {
    expect(toIsoDate(new Date(2026, 11, 5))).toBe('2026-12-05');
    expect(toIsoDate(parseIsoDate('2026-01-09') as Date)).toBe('2026-01-09');
  });
});

describe('DateField', () => {
  it('shows the value as dd/MM/yyyy with the accessible name and input id', async () => {
    const { fixture, el } = setup();
    await fixture.whenStable();
    fixture.detectChanges();
    const input = el.querySelector('input') as HTMLInputElement;
    expect(input.value).toBe('01/12/2026');
    expect(input.id).toBe('due');
    expect(input.getAttribute('aria-label')).toBe('Fecha de inicio');
    expect(input.placeholder).toBe('dd/mm/aaaa');
  });

  it('writes an ISO value to the form control when the date changes', () => {
    const { fixture, host, el } = setup();
    const component = fixture.debugElement.query((d) => d.name === 'app-date-field')
      .componentInstance as DateField;
    (component as unknown as { onDateChange: (d: Date | null) => void }).onDateChange(
      new Date(2026, 11, 15),
    );
    expect(host.control.value).toBe('2026-12-15');
    (component as unknown as { onDateChange: (d: Date | null) => void }).onDateChange(null);
    expect(host.control.value).toBeNull();
    expect(el).toBeTruthy();
  });

  it('follows the form control and disables with it', async () => {
    const { fixture, host, el } = setup();
    host.control.setValue('2026-12-20');
    fixture.detectChanges();
    await fixture.whenStable();
    fixture.detectChanges();
    expect((el.querySelector('input') as HTMLInputElement).value).toBe('20/12/2026');
    host.control.disable();
    fixture.detectChanges();
    expect((el.querySelector('input') as HTMLInputElement).disabled).toBe(true);
  });

  it('turns min and max into dates for the calendar', () => {
    const { fixture } = setup();
    const component = fixture.debugElement.query((d) => d.name === 'app-date-field')
      .componentInstance as unknown as {
      minDate: () => Date | null;
      maxDate: () => Date | null;
    };
    expect(toIsoDate(component.minDate() as Date)).toBe('2026-11-01');
    expect(toIsoDate(component.maxDate() as Date)).toBe('2026-12-31');
  });

  it('lays the picker out as a flex row so the input fills its container instead of collapsing', () => {
    const { el } = setup();
    document.body.appendChild(el);
    const picker = el.querySelector('p-datepicker') as HTMLElement;
    const host = el.querySelector('app-date-field') as HTMLElement;
    expect(getComputedStyle(picker).display).toBe('flex');
    expect(getComputedStyle(host).minWidth).toBe('0px');
    el.remove();
  });
});
