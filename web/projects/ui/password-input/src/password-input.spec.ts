import { ChangeDetectionStrategy, Component, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { FormControl, ReactiveFormsModule } from '@angular/forms';

import { PasswordInput } from './password-input';

@Component({
  imports: [PasswordInput, ReactiveFormsModule],
  template: `<app-password-input
    inputId="pwd"
    [formControl]="control"
    [invalid]="invalid()"
    [required]="true"
    describedBy="pwd-msg"
  />`,
  changeDetection: ChangeDetectionStrategy.OnPush,
})
class PasswordHost {
  readonly control = new FormControl('', { nonNullable: true });
  readonly invalid = signal(false);
}

function setup(): { el: HTMLElement; host: PasswordHost } {
  TestBed.configureTestingModule({ imports: [PasswordHost] });
  const fixture = TestBed.createComponent(PasswordHost);
  fixture.detectChanges();
  return { el: fixture.nativeElement as HTMLElement, host: fixture.componentInstance };
}

describe('PasswordInput', () => {
  it('renders a password field that is not visible by default', () => {
    const { el } = setup();
    const input = el.querySelector('input') as HTMLInputElement;
    expect(input.type).toBe('password');
    expect(input.getAttribute('autocomplete')).toBe('current-password');
    expect(input.getAttribute('aria-required')).toBe('true');
    expect(input.getAttribute('aria-describedby')).toBe('pwd-msg');
  });

  it('toggles visibility from a labelled button that reflects aria-pressed', () => {
    const { el } = setup();
    const input = el.querySelector('input') as HTMLInputElement;
    const toggle = el.querySelector('button') as HTMLButtonElement;
    expect(toggle.getAttribute('aria-label')).toBe('Mostrar contraseña');
    expect(toggle.getAttribute('aria-pressed')).toBe('false');
    expect(toggle.getAttribute('aria-controls')).toBe('pwd');

    toggle.click();
    TestBed.tick();
    expect(input.type).toBe('text');
    expect(toggle.getAttribute('aria-pressed')).toBe('true');

    toggle.click();
    TestBed.tick();
    expect(input.type).toBe('password');
    expect(toggle.getAttribute('aria-pressed')).toBe('false');
  });

  it('writes and reads the value through the form control', () => {
    const { el, host } = setup();
    const input = el.querySelector('input') as HTMLInputElement;
    host.control.setValue('Secreta123');
    TestBed.tick();
    expect(input.value).toBe('Secreta123');

    input.value = 'Otra123';
    input.dispatchEvent(new Event('input'));
    expect(host.control.value).toBe('Otra123');
  });

  it('marks the input invalid when the host says so', () => {
    const { el, host } = setup();
    const input = el.querySelector('input') as HTMLInputElement;
    expect(input.hasAttribute('aria-invalid')).toBe(false);
    host.invalid.set(true);
    TestBed.tick();
    expect(input.getAttribute('aria-invalid')).toBe('true');
  });
});
