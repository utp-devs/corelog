import { ComponentFixture, TestBed } from '@angular/core/testing';
import { provideRouter } from '@angular/router';
import { ApiError, PasswordResetApiService } from '@corelog/core';
import { Observable, of, throwError } from 'rxjs';
import { vi } from 'vitest';

import { ForgotPasswordPage } from './forgot-password-page';

function apiError(patch: Partial<ApiError>): ApiError {
  return { status: 500, code: 'unknown', message: 'boom', details: [], ...patch };
}

function setup(forgot: (email: string) => Observable<void>): {
  fixture: ComponentFixture<ForgotPasswordPage>;
  el: HTMLElement;
  forgot: ReturnType<typeof vi.fn>;
} {
  const fn = vi.fn(forgot);
  TestBed.configureTestingModule({
    imports: [ForgotPasswordPage],
    providers: [provideRouter([]), { provide: PasswordResetApiService, useValue: { forgot: fn } }],
  });
  const fixture = TestBed.createComponent(ForgotPasswordPage);
  fixture.detectChanges();
  return { fixture, el: fixture.nativeElement as HTMLElement, forgot: fn };
}

async function submitWith(fixture: ComponentFixture<ForgotPasswordPage>, email: string) {
  const el = fixture.nativeElement as HTMLElement;
  const input = el.querySelector<HTMLInputElement>('input[type="email"]');
  if (!input) throw new Error('email field missing');
  input.value = email;
  input.dispatchEvent(new Event('input'));
  el.querySelector<HTMLFormElement>('form')?.dispatchEvent(new Event('submit'));
  await fixture.whenStable();
  fixture.detectChanges();
}

describe('ForgotPasswordPage', () => {
  afterEach(() => {
    vi.useRealTimers();
    TestBed.resetTestingModule();
  });

  it('does not call the API with an invalid address', async () => {
    const { fixture, el, forgot } = setup(() => of(undefined));
    await submitWith(fixture, 'no-es-un-correo');
    expect(forgot).not.toHaveBeenCalled();
    expect(el.querySelector('[data-testid="forgot-sent"]')).toBeNull();
  });

  it('shows the generic confirmation and a countdown after sending', async () => {
    const { fixture, el, forgot } = setup(() => of(undefined));
    await submitWith(fixture, 'ana@example.com');
    expect(forgot).toHaveBeenCalledWith('ana@example.com');
    const sent = el.querySelector('[data-testid="forgot-sent"]');
    expect(sent?.textContent).toContain('Si el correo está registrado');
    const resend = el.querySelector<HTMLElement>('[data-testid="forgot-resend"]');
    expect(resend?.textContent).toContain('Reenviar en');
  });

  it('reads the same for an unknown address answered with an error', async () => {
    const { fixture, el } = setup(() => throwError(() => apiError({ status: 404 })));
    await submitWith(fixture, 'nadie@example.com');
    expect(el.querySelector('[data-testid="forgot-sent"]')?.textContent).toContain(
      'Si el correo está registrado',
    );
  });

  it('explains a rate limit with the wait and keeps the form', async () => {
    const { fixture, el } = setup(() =>
      throwError(() =>
        apiError({ status: 429, code: 'auth.rate_limited', retryAfterSeconds: 120 }),
      ),
    );
    await submitWith(fixture, 'ana@example.com');
    expect(el.querySelector('[role="alert"]')?.textContent).toContain('2 minutos');
    expect(el.querySelector('[data-testid="forgot-sent"]')).toBeNull();
  });

  it('counts down and allows another request', async () => {
    vi.useFakeTimers({ toFake: ['setInterval', 'clearInterval'] });
    const { fixture, el } = setup(() => of(undefined));
    await submitWith(fixture, 'ana@example.com');
    vi.advanceTimersByTime(61_000);
    fixture.detectChanges();
    expect(el.querySelector('[data-testid="forgot-resend"]')?.textContent).toContain(
      'Reenviar el correo',
    );
  });
});
