import { signal } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ActivatedRoute, convertToParamMap, provideRouter } from '@angular/router';
import { ApiError, AuthStore, PasswordResetApiService } from '@corelog/core';
import { Observable, Subject, of, throwError } from 'rxjs';
import { vi } from 'vitest';

import { VerifyEmailPage } from './verify-email-page';

function apiError(patch: Partial<ApiError>): ApiError {
  return {
    status: 400,
    code: 'auth.verification_token_invalid',
    message: 'x',
    details: [],
    ...patch,
  };
}

function setup(
  token: string | null,
  verify: () => Observable<void>,
  authenticated = false,
  requestVerification: () => Observable<void> = () => of(undefined),
): {
  fixture: ComponentFixture<VerifyEmailPage>;
  el: HTMLElement;
  verify: ReturnType<typeof vi.fn>;
} {
  const fn = vi.fn(verify);
  TestBed.configureTestingModule({
    imports: [VerifyEmailPage],
    providers: [
      provideRouter([]),
      {
        provide: ActivatedRoute,
        useValue: { snapshot: { queryParamMap: convertToParamMap(token ? { token } : {}) } },
      },
      { provide: PasswordResetApiService, useValue: { verify: fn, requestVerification } },
      { provide: AuthStore, useValue: { isAuthenticated: signal(authenticated) } },
    ],
  });
  const fixture = TestBed.createComponent(VerifyEmailPage);
  fixture.detectChanges();
  return { fixture, el: fixture.nativeElement as HTMLElement, verify: fn };
}

describe('VerifyEmailPage', () => {
  afterEach(() => TestBed.resetTestingModule());

  it('verifies on load and shows the loading state meanwhile', async () => {
    const pending = new Subject<void>();
    const { fixture, el, verify } = setup('tok', () => pending);
    expect(verify).toHaveBeenCalled();
    expect(el.querySelector('[role="status"]')).not.toBeNull();
    pending.next();
    pending.complete();
    await fixture.whenStable();
    fixture.detectChanges();
    expect(el.querySelector('[data-testid="verify-success"]')).not.toBeNull();
  });

  it('shows the error state without a token and does not call the API', () => {
    const { el, verify } = setup(null, () => of(undefined));
    expect(verify).not.toHaveBeenCalled();
    expect(el.querySelector('[data-testid="verify-error"]')).not.toBeNull();
  });

  it('shows the error state for an invalid token', async () => {
    const { fixture, el } = setup('tok', () => throwError(() => apiError({})));
    await fixture.whenStable();
    fixture.detectChanges();
    expect(el.querySelector('[data-testid="verify-error"]')).not.toBeNull();
    expect(el.querySelector('[data-testid="verify-resend"]')).toBeNull();
  });

  it('lets a signed-in person ask for a new email', async () => {
    const { fixture, el } = setup('tok', () => throwError(() => apiError({})), true);
    await fixture.whenStable();
    fixture.detectChanges();
    el.querySelector<HTMLElement>('[data-testid="verify-resend"] button')?.click();
    await fixture.whenStable();
    fixture.detectChanges();
    expect(el.textContent).toContain('Te enviamos un correo nuevo');
  });
});
