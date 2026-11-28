import { signal, type WritableSignal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { ActivatedRoute, Router, convertToParamMap, provideRouter } from '@angular/router';
import { ApiError, AuthStore, HOME_URL, LOCATION_ASSIGN } from '@corelog/core';
import { vi } from 'vitest';

import { LoginPage } from './login-page';

function apiError(patch: Partial<ApiError>): ApiError {
  return { status: 422, code: 'auth.two_factor_invalid', message: 'x', details: [], ...patch };
}

function setup(
  make: (challenge: WritableSignal<string | null>) => ReturnType<typeof vi.fn> = () =>
    vi.fn().mockResolvedValue(undefined),
) {
  const twoFactorChallenge = signal<string | null>('challenge-1');
  const completeTwoFactor = make(twoFactorChallenge);
  const cancelTwoFactor = vi.fn(() => twoFactorChallenge.set(null));
  const assign = vi.fn<(url: string) => void>();
  TestBed.configureTestingModule({
    imports: [LoginPage],
    providers: [
      provideRouter([]),
      { provide: ActivatedRoute, useValue: { snapshot: { queryParamMap: convertToParamMap({}) } } },
      {
        provide: AuthStore,
        useValue: { login: vi.fn(), twoFactorChallenge, completeTwoFactor, cancelTwoFactor },
      },
      { provide: LOCATION_ASSIGN, useValue: assign },
      { provide: HOME_URL, useValue: '/workspace/' },
    ],
  });
  vi.spyOn(TestBed.inject(Router), 'navigateByUrl').mockResolvedValue(true);
  const fixture = TestBed.createComponent(LoginPage);
  fixture.detectChanges();
  const el = fixture.nativeElement as HTMLElement;
  const code = (): HTMLInputElement =>
    el.querySelector<HTMLInputElement>('[data-testid="two-factor-code"]') as HTMLInputElement;
  const type = (value: string): void => {
    code().value = value;
    code().dispatchEvent(new Event('input'));
  };
  const submit = async (): Promise<void> => {
    el.querySelector<HTMLButtonElement>('button[type="submit"]')?.click();
    await fixture.whenStable();
    fixture.detectChanges();
  };
  return {
    fixture,
    el,
    code,
    type,
    submit,
    completeTwoFactor,
    cancelTwoFactor,
    assign,
    twoFactorChallenge,
  };
}

describe('LoginPage second step', () => {
  it('shows the code step instead of the credentials when a challenge is waiting', () => {
    const { el } = setup();
    expect(el.querySelector('[data-testid="two-factor-step"]')).not.toBeNull();
    expect(el.querySelector('input[type="email"]')).toBeNull();
    expect(el.textContent).toContain('Verificación en dos pasos');
  });

  it('sends the code and leaves for the workspace', async () => {
    const { type, submit, completeTwoFactor, assign } = setup();
    type('123456');
    await submit();
    expect(completeTwoFactor).toHaveBeenCalledWith({ code: '123456' });
    expect(assign).toHaveBeenCalledWith('/workspace/');
  });

  it('sends a recovery code after switching, and explains a rejected code', async () => {
    const { el, type, submit, completeTwoFactor } = setup(() =>
      vi.fn().mockRejectedValue(apiError({ code: 'auth.two_factor_invalid' })),
    );
    el.querySelector<HTMLButtonElement>('[data-testid="two-factor-toggle"]')?.click();
    type('abcde-12345');
    await submit();
    expect(completeTwoFactor).toHaveBeenCalledWith({ recoveryCode: 'abcde-12345' });
    expect(el.textContent).toContain('El código no es válido o ya se usó');
  });

  it('returns to the credentials with a message when the challenge expired', async () => {
    const { el, fixture, type, submit } = setup((challenge) =>
      vi.fn(() => {
        challenge.set(null);
        return Promise.reject(apiError({ status: 401, code: 'auth.challenge_expired' }));
      }),
    );
    type('123456');
    await submit();
    fixture.detectChanges();
    expect(el.querySelector('input[type="email"]')).not.toBeNull();
    expect(el.textContent).toContain('La verificación caducó');
  });

  it('goes back to the credentials on request', () => {
    const { el, fixture, cancelTwoFactor } = setup();
    const back = Array.from(el.querySelectorAll('button')).find((b) =>
      b.textContent?.includes('Volver al inicio de sesión'),
    );
    back?.click();
    fixture.detectChanges();
    expect(cancelTwoFactor).toHaveBeenCalled();
    expect(el.querySelector('input[type="email"]')).not.toBeNull();
  });
});
