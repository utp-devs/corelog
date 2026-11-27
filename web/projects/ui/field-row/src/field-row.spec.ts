import { Component, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { FormControl, ReactiveFormsModule, Validators } from '@angular/forms';
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

@Component({
  imports: [FieldRow, FieldControl, ReactiveFormsModule],
  template: `
    <app-field-row
      label="Título"
      [control]="title"
      [messages]="{ minlength: 'El título es demasiado corto' }"
      [submitted]="submitted()"
    >
      <input appFieldControl type="text" [formControl]="title" />
    </app-field-row>
    <app-field-row label="Bloqueado" disabledReason="Solo un administrador puede cambiarlo">
      <input appFieldControl type="text" disabled />
    </app-field-row>
    <app-field-row label="Notas" [optional]="true">
      <input appFieldControl type="text" />
    </app-field-row>
  `,
})
class FieldRowControlHost {
  readonly title = new FormControl('', [Validators.minLength(3)]);
  readonly submitted = signal(false);
}

describe('FieldRow error timing', () => {
  function setup() {
    TestBed.configureTestingModule({ imports: [FieldRowControlHost] });
    const fixture = TestBed.createComponent(FieldRowControlHost);
    fixture.detectChanges();
    return { fixture, el: fixture.nativeElement as HTMLElement };
  }

  it('shows no error on an empty untouched field', () => {
    const { el } = setup();
    expect(el.textContent).not.toContain('demasiado corto');
    expect(inputFor(el, 'Título').hasAttribute('aria-invalid')).toBe(false);
  });

  it('shows the error with an icon once the field was edited and touched', () => {
    const { fixture, el } = setup();
    const host = fixture.componentInstance;
    host.title.setValue('ab');
    host.title.markAsDirty();
    fixture.detectChanges();
    expect(el.textContent).toContain('El título es demasiado corto');
    expect(el.querySelector('.field-row__text--error .pi-exclamation-circle')).not.toBeNull();
    expect(inputFor(el, 'Título').getAttribute('aria-invalid')).toBe('true');
  });

  it('shows the error of an untouched invalid field after submit', () => {
    const { fixture, el } = setup();
    fixture.componentInstance.title.setValue('ab');
    fixture.detectChanges();
    expect(el.textContent).not.toContain('demasiado corto');
    fixture.componentInstance.submitted.set(true);
    fixture.detectChanges();
    expect(el.textContent).toContain('El título es demasiado corto');
  });

  it('explains why a control is disabled and links it through aria-describedby', () => {
    const { el } = setup();
    const input = inputFor(el, 'Bloqueado');
    const described = document.getElementById(input.getAttribute('aria-describedby') ?? '');
    expect(described?.textContent).toContain('Solo un administrador puede cambiarlo');
    expect(described?.querySelector('.pi-lock')).not.toBeNull();
  });

  it('marks optional fields with a muted hint', () => {
    const { el } = setup();
    expect(el.querySelector('.field-row__optional')?.textContent).toContain('(opcional)');
  });
});
