import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ActivatedRoute, convertToParamMap, provideRouter, Router } from '@angular/router';
import { AuthStore, HOME_URL, LOCATION_ASSIGN, RegisterPayload } from '@corelog/core';
import { vi } from 'vitest';

import { RegisterPage } from './register-page';

interface Setup {
  fixture: ComponentFixture<RegisterPage>;
  el: HTMLElement;
  register: ReturnType<typeof vi.fn<(payload: RegisterPayload) => Promise<void>>>;
  assign: ReturnType<typeof vi.fn<(url: string) => void>>;
}

function setup(): Setup {
  const register = vi
    .fn<(payload: RegisterPayload) => Promise<void>>()
    .mockResolvedValue(undefined);
  const assign = vi.fn<(url: string) => void>();
  TestBed.configureTestingModule({
    imports: [RegisterPage],
    providers: [
      provideRouter([]),
      { provide: ActivatedRoute, useValue: { snapshot: { queryParamMap: convertToParamMap({}) } } },
      { provide: AuthStore, useValue: { register } },
      { provide: LOCATION_ASSIGN, useValue: assign },
      { provide: HOME_URL, useValue: '/workspace/' },
    ],
  });
  vi.spyOn(TestBed.inject(Router), 'navigateByUrl').mockResolvedValue(true);
  const fixture = TestBed.createComponent(RegisterPage);
  fixture.detectChanges();
  return { fixture, el: fixture.nativeElement as HTMLElement, register, assign };
}

function type(el: HTMLElement, selector: string, value: string): void {
  const input = el.querySelector<HTMLInputElement>(selector);
  if (!input) throw new Error(`missing ${selector}`);
  input.value = value;
  input.dispatchEvent(new Event('input'));
}

function submit(el: HTMLElement): void {
  el.querySelector<HTMLButtonElement>('button[type="submit"]')?.click();
}

async function settle(fixture: ComponentFixture<RegisterPage>): Promise<void> {
  await fixture.whenStable();
  fixture.detectChanges();
}

describe('RegisterPage', () => {
  it('explains the minimum length of the name and password in Spanish', async () => {
    const { fixture, el, register } = setup();
    type(el, '#register-name', 'A');
    type(el, '#register-password', 'corta');
    submit(el);
    await settle(fixture);
    expect(el.textContent).toContain('El nombre debe tener al menos 2 caracteres');
    expect(el.textContent).toContain('La contraseña debe tener al menos 8 caracteres');
    expect(register).not.toHaveBeenCalled();
  });

  it('creates the account when every field is valid', async () => {
    const { fixture, el, register } = setup();
    type(el, '#register-name', 'Ana Torres');
    type(el, '#register-email', 'ana@corelog.test');
    type(el, '#register-password', 'Secreta123');
    submit(el);
    await settle(fixture);
    expect(register).toHaveBeenCalledWith({
      name: 'Ana Torres',
      email: 'ana@corelog.test',
      password: 'Secreta123',
    });
  });
});

describe('RegisterPage form behaviour', () => {
  it('uses the autocomplete tokens for name, email and new password', () => {
    const { el } = setup();
    expect(el.querySelector('#register-name')?.getAttribute('autocomplete')).toBe('name');
    expect(el.querySelector('#register-email')?.getAttribute('autocomplete')).toBe('email');
    expect(el.querySelector('#register-password')?.getAttribute('autocomplete')).toBe(
      'new-password',
    );
  });

  it('announces the password strength as the user types', () => {
    const { fixture, el } = setup();
    type(el, '#register-password', 'Secreta123!456');
    fixture.detectChanges();
    expect(el.textContent).toContain('Seguridad de la contraseña: Fuerte');
  });

  it('shows the terms line', () => {
    const { el } = setup();
    expect(el.textContent).toMatch(/condiciones de uso y la política de privacidad/);
  });

  it('shows the password helper until the field has an error', async () => {
    const { fixture, el } = setup();
    expect(el.textContent).toContain('Mínimo 8 caracteres.');
    submit(el);
    await settle(fixture);
    expect(el.textContent).not.toContain('Mínimo 8 caracteres.');
  });
});
