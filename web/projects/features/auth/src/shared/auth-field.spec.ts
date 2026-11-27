import { Component, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { FormControl, ReactiveFormsModule, Validators } from '@angular/forms';

import { AuthControl, AuthField } from './auth-field';

@Component({
  imports: [AuthField, AuthControl, ReactiveFormsModule],
  template: `
    <app-auth-field
      label="Correo electrónico"
      icon="pi-envelope"
      helper="Usa tu correo de trabajo"
      [required]="true"
      [control]="control"
      [messages]="{ required: 'Obligatorio' }"
      [submitted]="submitted()"
      [error]="serverError()"
    >
      <input appAuthControl type="email" [formControl]="control" />
    </app-auth-field>
  `,
})
class Host {
  readonly control = new FormControl('', { nonNullable: true, validators: [Validators.required] });
  readonly submitted = signal(false);
  readonly serverError = signal<string | undefined>(undefined);
}

function setup() {
  TestBed.configureTestingModule({ imports: [Host] });
  const fixture = TestBed.createComponent(Host);
  fixture.detectChanges();
  const el = fixture.nativeElement as HTMLElement;
  return { fixture, el, host: fixture.componentInstance };
}

describe('AuthField', () => {
  afterEach(() => TestBed.resetTestingModule());

  it('ties a real label to the projected input and marks it required', () => {
    const { el } = setup();
    const label = el.querySelector('label') as HTMLLabelElement;
    const input = el.querySelector('input') as HTMLInputElement;
    expect(label.htmlFor).toBe(input.id);
    expect(label.textContent).toContain('Correo electrónico');
    expect(label.textContent).toContain('(obligatorio)');
    expect(input.getAttribute('aria-required')).toBe('true');
  });

  it('draws the icon on the right and keeps it out of the accessibility tree', () => {
    const { el } = setup();
    const icon = el.querySelector('.auth-field__icon');
    expect(icon?.classList.contains('pi-envelope')).toBe(true);
    expect(icon?.getAttribute('aria-hidden')).toBe('true');
  });

  it('shows the helper until an error may be shown', () => {
    const { el } = setup();
    const input = el.querySelector('input') as HTMLInputElement;
    expect(el.textContent).toContain('Usa tu correo de trabajo');
    expect(input.getAttribute('aria-invalid')).toBeNull();
    const described = document.getElementById(input.getAttribute('aria-describedby') ?? '');
    expect(described?.textContent).toContain('Usa tu correo');
  });

  it('shows the control error after submit and links it with aria-describedby', () => {
    const { fixture, el, host } = setup();
    host.submitted.set(true);
    fixture.detectChanges();
    const input = el.querySelector('input') as HTMLInputElement;
    expect(input.getAttribute('aria-invalid')).toBe('true');
    const described = document.getElementById(input.getAttribute('aria-describedby') ?? '');
    expect(described?.textContent).toContain('Obligatorio');
  });

  it('shows the error once the control was touched', () => {
    const { fixture, el, host } = setup();
    host.control.markAsTouched();
    fixture.detectChanges();
    expect(el.querySelector('.auth-field__text--error')?.textContent).toContain('Obligatorio');
  });

  it('lets an explicit server message win over the helper', () => {
    const { fixture, el, host } = setup();
    host.control.setValue('ana@corelog.test');
    host.serverError.set('Ya existe una cuenta con este correo');
    fixture.detectChanges();
    expect(el.querySelector('.auth-field__text--error')?.textContent).toContain('Ya existe');
    expect(el.textContent).not.toContain('Usa tu correo de trabajo');
  });
});
