import { signal } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { HttpErrorResponse } from '@angular/common/http';
import { AuthStore, PasswordResetApiService, type User } from '@corelog/core';
import { provideCorelogTesting } from '@corelog/core/testing';
import { Observable, of, throwError } from 'rxjs';

import { VerifyEmailBanner } from './verify-email-banner';

function user(verified: string | null): User {
  return {
    id: 'u-1',
    name: 'Ana',
    email: 'ana@corelog.test',
    role: 'user',
    teamId: null,
    displayName: 'Ana',
    jobTitle: '',
    department: '',
    avatarKey: null,
    timezone: 'UTC',
    createdAt: '2026-11-01T00:00:00Z',
    lastLoginAt: null,
    emailVerifiedAt: verified,
  };
}

function setup(
  current: User | null,
  request: () => Observable<void> = () => of(undefined),
): {
  fixture: ComponentFixture<VerifyEmailBanner>;
  el: HTMLElement;
  request: ReturnType<typeof vi.fn>;
} {
  const requestVerification = vi.fn(request);
  TestBed.configureTestingModule({
    providers: [
      provideCorelogTesting(),
      { provide: AuthStore, useValue: { user: signal(current) } },
      { provide: PasswordResetApiService, useValue: { requestVerification } },
    ],
  });
  const fixture = TestBed.createComponent(VerifyEmailBanner);
  fixture.detectChanges();
  return { fixture, el: fixture.nativeElement as HTMLElement, request: requestVerification };
}

const BANNER = '[data-testid="verify-email-banner"]';

describe('VerifyEmailBanner', () => {
  beforeEach(() => sessionStorage.clear());
  afterEach(() => {
    TestBed.resetTestingModule();
    sessionStorage.clear();
  });

  it('is shown when the email is not verified', () => {
    const { el } = setup(user(null));
    expect(el.querySelector(BANNER)?.textContent).toContain('ana@corelog.test');
  });

  it('is hidden when the email is verified or nobody is signed in', () => {
    expect(setup(user('2026-11-02T00:00:00Z')).el.querySelector(BANNER)).toBeNull();
    TestBed.resetTestingModule();
    expect(setup(null).el.querySelector(BANNER)).toBeNull();
  });

  it('sends the verification email again and confirms it', async () => {
    const { fixture, el, request } = setup(user(null));
    el.querySelector<HTMLButtonElement>('[data-testid="verify-email-resend"]')?.click();
    await fixture.whenStable();
    fixture.detectChanges();
    expect(request).toHaveBeenCalledTimes(1);
    expect(el.querySelector('[data-testid="verify-email-feedback"]')?.textContent).toContain(
      'Te enviamos un correo',
    );
  });

  it('explains a rate limit answer', async () => {
    const { fixture, el } = setup(user(null), () =>
      throwError(() => new HttpErrorResponse({ status: 429 })),
    );
    el.querySelector<HTMLButtonElement>('[data-testid="verify-email-resend"]')?.click();
    await fixture.whenStable();
    fixture.detectChanges();
    const feedback = el.querySelector('[data-testid="verify-email-feedback"]');
    expect(feedback?.textContent).toContain('demasiados correos');
    expect(feedback?.getAttribute('role')).toBe('alert');
  });

  it('hides itself when the API says the email is already verified', async () => {
    const { fixture, el } = setup(user(null), () =>
      throwError(
        () =>
          new HttpErrorResponse({
            status: 409,
            error: { error: { code: 'auth.email_already_verified', message: 'ya verificado' } },
          }),
      ),
    );
    el.querySelector<HTMLButtonElement>('[data-testid="verify-email-resend"]')?.click();
    await fixture.whenStable();
    fixture.detectChanges();
    expect(el.querySelector(BANNER)).toBeNull();
  });

  it('can be dismissed for the session', () => {
    const { fixture, el } = setup(user(null));
    el.querySelector<HTMLButtonElement>('[data-testid="verify-email-dismiss"]')?.click();
    fixture.detectChanges();
    expect(el.querySelector(BANNER)).toBeNull();
    TestBed.resetTestingModule();
    expect(setup(user(null)).el.querySelector(BANNER)).toBeNull();
  });
});
