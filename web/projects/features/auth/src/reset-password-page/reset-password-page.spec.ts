import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ActivatedRoute, convertToParamMap, provideRouter } from '@angular/router';
import { ApiError, PasswordResetApiService, ResetPasswordPayload } from '@corelog/core';
import { Observable, of, throwError } from 'rxjs';
import { vi } from 'vitest';

import { ResetPasswordPage } from './reset-password-page';

function apiError(patch: Partial<ApiError>): ApiError {
  return { status: 422, code: 'unknown', message: 'x', details: [], ...patch };
}

function setup(
  token: string | null,
  reset: (payload: ResetPasswordPayload) => Observable<void> = () => of(undefined),
): {
  fixture: ComponentFixture<ResetPasswordPage>;
  el: HTMLElement;
  reset: ReturnType<typeof vi.fn>;
} {
  const fn = vi.fn(reset);
  TestBed.configureTestingModule({
    imports: [ResetPasswordPage],
    providers: [
      provideRouter([]),
      {
        provide: ActivatedRoute,
        useValue: {
          snapshot: { queryParamMap: convertToParamMap(token ? { token } : {}) },
        },
      },
      { provide: PasswordResetApiService, useValue: { reset: fn } },
    ],
  });
  const fixture = TestBed.createComponent(ResetPasswordPage);
  fixture.detectChanges();
  return { fixture, el: fixture.nativeElement as HTMLElement, reset: fn };
}

async function fillAndSubmit(
  fixture: ComponentFixture<ResetPasswordPage>,
  password: string,
  confirmation: string,
) {
  const el = fixture.nativeElement as HTMLElement;
  const inputs = el.querySelectorAll<HTMLInputElement>('input[type="password"]');
  inputs[0].value = password;
  inputs[0].dispatchEvent(new Event('input'));
  inputs[1].value = confirmation;
  inputs[1].dispatchEvent(new Event('input'));
  el.querySelector<HTMLFormElement>('form')?.dispatchEvent(new Event('submit'));
  await fixture.whenStable();
  fixture.detectChanges();
}

describe('ResetPasswordPage', () => {
  afterEach(() => TestBed.resetTestingModule());

  it('shows the invalid-link state without a token', () => {
    const { el } = setup(null);
    expect(el.querySelector('[data-testid="reset-invalid"]')).not.toBeNull();
    expect(el.querySelector('[data-testid="reset-again"]')?.getAttribute('href')).toBe(
      '/forgot-password',
    );
  });

  it('blocks a mismatched confirmation before calling the API', async () => {
    const { fixture, el, reset } = setup('tok');
    await fillAndSubmit(fixture, 'Clave-muy-larga-1', 'otra-cosa');
    expect(reset).not.toHaveBeenCalled();
    expect(el.textContent).toContain('Las contraseñas no coinciden');
  });

  it('sends the token and shows the success state with a link to sign in', async () => {
    const { fixture, el, reset } = setup('tok');
    await fillAndSubmit(fixture, 'Clave-muy-larga-1', 'Clave-muy-larga-1');
    expect(reset).toHaveBeenCalledWith({
      token: 'tok',
      password: 'Clave-muy-larga-1',
      passwordConfirmation: 'Clave-muy-larga-1',
    });
    expect(el.querySelector('[data-testid="reset-done"]')).not.toBeNull();
    expect(el.querySelector('[data-testid="reset-login"]')?.getAttribute('href')).toBe('/login');
  });

  it('maps a weak password to the field and keeps the form', async () => {
    const { fixture, el } = setup('tok', () =>
      throwError(() => apiError({ code: 'user.password_weak' })),
    );
    await fillAndSubmit(fixture, 'Clave-muy-larga-1', 'Clave-muy-larga-1');
    expect(el.textContent).toContain('demasiado débil');
    expect(el.querySelector('[data-testid="reset-done"]')).toBeNull();
  });

  it('shows the invalid-link state when the server rejects the token', async () => {
    const { fixture, el } = setup('tok', () =>
      throwError(() => apiError({ status: 400, code: 'auth.reset_token_invalid' })),
    );
    await fillAndSubmit(fixture, 'Clave-muy-larga-1', 'Clave-muy-larga-1');
    expect(el.querySelector('[data-testid="reset-invalid"]')).not.toBeNull();
  });
});
