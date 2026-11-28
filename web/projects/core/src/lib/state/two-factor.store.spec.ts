import { provideHttpClient } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';

import { provideCorelogTesting } from '../../testing';
import { AuthStore } from './auth.store';
import { TwoFactorStore } from './two-factor.store';

const CODES = Array.from({ length: 10 }, (_, i) => `CODE-${i}`);

describe('TwoFactorStore', () => {
  let store: TwoFactorStore;
  let http: HttpTestingController;

  beforeEach(() => {
    TestBed.configureTestingModule({
      providers: [provideCorelogTesting(), provideHttpClient(), provideHttpClientTesting()],
    });
    store = TestBed.inject(TwoFactorStore);
    http = TestBed.inject(HttpTestingController);
  });

  afterEach(() => http.verify());

  it('loads the status', () => {
    store.load();
    http.expectOne('/api/v1/auth/2fa').flush({
      data: { enabled: true, enabled_at: '2026-01-01T00:00:00Z', recovery_codes_remaining: 7 },
    });
    expect(store.enabled()).toBe(true);
    expect(store.status.value()?.recoveryCodesRemaining).toBe(7);
  });

  it('enrolls, confirms with a code and keeps the recovery codes in memory only', async () => {
    const begin = store.beginEnrollment();
    http
      .expectOne('/api/v1/auth/2fa/enroll')
      .flush({ data: { secret: 'JBSWY3DP', otpauth_uri: 'otpauth://totp/x?secret=JBSWY3DP' } });
    expect((await begin).secret).toBe('JBSWY3DP');
    expect(store.enrollment()?.otpauthUri).toContain('otpauth://');

    const confirm = store.confirmEnrollment('123456');
    const request = http.expectOne('/api/v1/auth/2fa/confirm');
    expect(request.request.body).toEqual({ code: '123456' });
    request.flush({ data: { recovery_codes: CODES } });
    await confirm;
    http
      .expectOne('/api/v1/auth/2fa')
      .flush({ data: { enabled: true, enabled_at: null, recovery_codes_remaining: 10 } });

    expect(store.recoveryCodes()).toEqual(CODES);
    expect(store.enrollment()).toBeNull();
    expect(JSON.stringify({ ...localStorage })).not.toContain('CODE-0');

    store.clearRecoveryCodes();
    expect(store.recoveryCodes()).toBeNull();
  });

  it('rejects with the API error when the code is wrong and keeps the enrollment', async () => {
    const begin = store.beginEnrollment();
    http
      .expectOne('/api/v1/auth/2fa/enroll')
      .flush({ data: { secret: 'S', otpauth_uri: 'otpauth://x' } });
    await begin;
    const confirm = store.confirmEnrollment('000000');
    http
      .expectOne('/api/v1/auth/2fa/confirm')
      .flush(
        { error: { code: 'auth.two_factor_invalid', message: 'El codigo no es valido' } },
        { status: 422, statusText: 'Unprocessable' },
      );
    await expect(confirm).rejects.toMatchObject({ code: 'auth.two_factor_invalid' });
    expect(store.enrollment()).not.toBeNull();
    expect(store.recoveryCodes()).toBeNull();
  });

  it('disables with the password and a code, then refreshes the status', async () => {
    const disable = store.disable({ password: 'secret', code: '123456' });
    const request = http.expectOne('/api/v1/auth/2fa/disable');
    expect(request.request.body).toEqual({ password: 'secret', code: '123456' });
    request.flush(null, { status: 204, statusText: 'No Content' });
    await disable;
    http
      .expectOne('/api/v1/auth/2fa')
      .flush({ data: { enabled: false, enabled_at: null, recovery_codes_remaining: 0 } });
    expect(store.enabled()).toBe(false);
  });
});

describe('AuthStore two-step sign-in', () => {
  let auth: AuthStore;
  let http: HttpTestingController;
  const user = {
    id: 'u1',
    name: 'Ana',
    email: 'ana@example.com',
    role: 'user',
    created_at: '2026-01-01T00:00:00Z',
  };

  beforeEach(() => {
    TestBed.configureTestingModule({
      providers: [provideCorelogTesting(), provideHttpClient(), provideHttpClientTesting()],
    });
    auth = TestBed.inject(AuthStore);
    http = TestBed.inject(HttpTestingController);
  });

  afterEach(() => http.verify());

  async function reachChallenge(): Promise<void> {
    const login = auth.login({ email: 'ana@example.com', password: 'pw' });
    http
      .expectOne('/api/v1/auth/login')
      .flush({ data: { two_factor_required: true, challenge_token: 'challenge-1' } });
    await login;
  }

  it('waits for the second step instead of opening the session', async () => {
    await reachChallenge();
    expect(auth.twoFactorChallenge()).toBe('challenge-1');
    expect(auth.isAuthenticated()).toBe(false);
  });

  it('finishes the sign-in with a code', async () => {
    await reachChallenge();
    const verify = auth.completeTwoFactor({ code: '123456' });
    const request = http.expectOne('/api/v1/auth/login/2fa');
    expect(request.request.body).toEqual({ challenge_token: 'challenge-1', code: '123456' });
    request.flush({ data: { token: 't', user } });
    await verify;
    expect(auth.isAuthenticated()).toBe(true);
    expect(auth.twoFactorChallenge()).toBeNull();
  });

  it('keeps the challenge after a wrong code and drops it when it expired', async () => {
    await reachChallenge();
    const wrong = auth.completeTwoFactor({ recoveryCode: 'bad' });
    http
      .expectOne('/api/v1/auth/login/2fa')
      .flush(
        { error: { code: 'auth.two_factor_invalid', message: 'x' } },
        { status: 422, statusText: 'Unprocessable' },
      );
    await expect(wrong).rejects.toMatchObject({ code: 'auth.two_factor_invalid' });
    expect(auth.twoFactorChallenge()).toBe('challenge-1');

    const late = auth.completeTwoFactor({ code: '111111' });
    http
      .expectOne('/api/v1/auth/login/2fa')
      .flush(
        { error: { code: 'auth.challenge_expired', message: 'x' } },
        { status: 401, statusText: 'Unauthorized' },
      );
    await expect(late).rejects.toMatchObject({ code: 'auth.challenge_expired' });
    expect(auth.twoFactorChallenge()).toBeNull();
  });
});
