import { Component, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { FormControl, ReactiveFormsModule } from '@angular/forms';

import { SELECT_FILTER_THRESHOLD, SelectField, SelectFieldOption } from './select-field';

function optionsOf(count: number): SelectFieldOption[] {
  return Array.from({ length: count }, (_, i) => ({ value: `v${i}`, label: `Opción ${i}` }));
}

@Component({
  imports: [SelectField, ReactiveFormsModule],
  template: `<app-select-field
    ariaLabel="Rol"
    [formControl]="control"
    [options]="options()"
    [size]="size()"
    [disabledReason]="reason"
  />`,
})
class SelectHost {
  readonly control = new FormControl<string | null>('v1');
  readonly options = signal(optionsOf(3));
  readonly size = signal<'default' | 'compact'>('default');
  readonly reason = 'Solo un administrador puede cambiar el rol.';
}

function setup() {
  TestBed.configureTestingModule({ imports: [SelectHost] });
  const fixture = TestBed.createComponent(SelectHost);
  fixture.detectChanges();
  const select = fixture.debugElement.query((d) => d.name === 'app-select-field')
    .componentInstance as SelectField;
  return {
    fixture,
    host: fixture.componentInstance,
    el: fixture.nativeElement as HTMLElement,
    select,
  };
}

describe('SelectField', () => {
  it('shows the label of the selected value with an accessible name', async () => {
    const { fixture, el } = setup();
    await fixture.whenStable();
    fixture.detectChanges();
    expect(el.querySelector('[role="combobox"]')?.getAttribute('aria-label')).toBe('Rol');
    expect(el.textContent).toContain('Opción 1');
  });

  it('turns the filter box on only for long lists', () => {
    const { fixture, host, select } = setup();
    const hasFilter = () => (select as unknown as { hasFilter: () => boolean }).hasFilter();
    expect(hasFilter()).toBe(false);
    host.options.set(optionsOf(SELECT_FILTER_THRESHOLD + 1));
    fixture.detectChanges();
    expect(hasFilter()).toBe(true);
  });

  it('marks the compact variant for table cells', () => {
    const { fixture, host, el } = setup();
    expect(el.querySelector('.select-field--compact')).toBeNull();
    host.size.set('compact');
    fixture.detectChanges();
    expect(el.querySelector('.select-field--compact')).not.toBeNull();
  });

  it('writes the chosen value to the form control', () => {
    const { host, select } = setup();
    (select as unknown as { onValueChange: (v: string | null) => void }).onValueChange('v2');
    expect(host.control.value).toBe('v2');
  });

  it('explains why it is disabled through its tooltip', () => {
    const { fixture, host, el } = setup();
    host.control.disable();
    fixture.detectChanges();
    expect(el.querySelector('p-select')?.getAttribute('title')).toBe(host.reason);
  });
});
