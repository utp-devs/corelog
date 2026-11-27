import { Component, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';

import { FieldControl, FieldRow } from './field-row';

@Component({
  imports: [FieldRow, FieldControl],
  template: `
    <app-field-row label="Nombre">
      <input appFieldControl type="text" />
    </app-field-row>
    <app-field-row label="Clave" [required]="true">
      <input appFieldControl type="text" />
    </app-field-row>
    <app-field-row label="Descripción">
      <input appFieldControl type="text" />
    </app-field-row>
    <app-field-row label="Clave de proyecto" [helper]="helper()" [error]="error()">
      <input appFieldControl type="text" />
    </app-field-row>
    <app-field-row label="Estado">
      <span>Activo</span>
    </app-field-row>
    <app-field-row label="A" [variant]="variant()">x</app-field-row>
  `,
})
class FieldRowHost {
  readonly helper = signal<string | undefined>(undefined);
  readonly error = signal<string | undefined>(undefined);
  readonly variant = signal<'stacked' | 'inline'>('stacked');
}

function create(): {
  fixture: ReturnType<typeof TestBed.createComponent<FieldRowHost>>;
  el: HTMLElement;
} {
  TestBed.configureTestingModule({ imports: [FieldRowHost] });
  const fixture = TestBed.createComponent(FieldRowHost);
  fixture.detectChanges();
  return { fixture, el: fixture.nativeElement as HTMLElement };
}

function inputFor(el: HTMLElement, label: string): HTMLInputElement {
  const labelEl = Array.from(el.querySelectorAll('label')).find((l) =>
    l.textContent?.trim().startsWith(label),
  );
  return el.querySelector(`#${labelEl?.getAttribute('for')}`) as HTMLInputElement;
}

describe('FieldRow', () => {
  it('associates the label with the control', () => {
    const { el } = create();
    expect(inputFor(el, 'Nombre')).not.toBeNull();
  });

  it('marks required fields with aria-required and a spoken "(obligatorio)" hint', () => {
    const { el } = create();
    const input = inputFor(el, 'Clave');
    expect(input.getAttribute('aria-required')).toBe('true');
    expect(el.textContent).toContain('(obligatorio)');
  });

  it('does not set aria-required on optional fields', () => {
    const { el } = create();
    expect(inputFor(el, 'Descripción').hasAttribute('aria-required')).toBe(false);
  });

  it('links helper text to the control through aria-describedby', () => {
    const { fixture, el } = create();
    fixture.componentInstance.helper.set('Tres letras mayúsculas');
    fixture.detectChanges();
    const input = inputFor(el, 'Clave de proyecto');
    const described = document.getElementById(input.getAttribute('aria-describedby') ?? '');
    expect(described?.textContent).toBe('Tres letras mayúsculas');
  });

  it('shows the error instead of the helper and marks the control invalid', () => {
    const { fixture, el } = create();
    fixture.componentInstance.helper.set('Tres letras mayúsculas');
    fixture.componentInstance.error.set('La clave ya existe');
    fixture.detectChanges();
    const input = inputFor(el, 'Clave de proyecto');
    expect(input.getAttribute('aria-invalid')).toBe('true');
    const described = document.getElementById(input.getAttribute('aria-describedby') ?? '');
    expect(described?.textContent).toBe('La clave ya existe');
    expect(el.textContent).not.toContain('Tres letras mayúsculas');
  });

  it('accepts a plain node as control, without injected attributes', () => {
    const { el } = create();
    expect(el.textContent).toContain('Activo');
  });

  it('exposes the variant as data-variant (stacked by default, inline for detail panels)', () => {
    const { fixture, el } = create();
    const row = () => el.querySelectorAll('[data-variant]')[5];
    expect(row()?.getAttribute('data-variant')).toBe('stacked');
    fixture.componentInstance.variant.set('inline');
    fixture.detectChanges();
    expect(row()?.getAttribute('data-variant')).toBe('inline');
  });
});
