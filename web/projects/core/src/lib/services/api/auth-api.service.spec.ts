import { provideHttpClient, withInterceptors } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';

import { provideCorelogTesting } from '../../../testing';
import { authInterceptor } from '../../interceptors/auth.interceptor';
import { errorInterceptor } from '../../interceptors/error.interceptor';
import { SessionService } from '../session.service';
import { AuthApiService } from './auth-api.service';

const USER_RESPONSE = {
  id: 'u-1',
  name: 'Ada',
  email: 'ada@example.com',
  role: 'user',
  created_at: '2026-01-01T00:00:00Z',
};

function setup(): { service: AuthApiService; http: HttpTestingController } {
  TestBed.configureTestingModule({
    providers: [
      provideCorelogTesting(),
      provideHttpClient(withInterceptors([authInterceptor, errorInterceptor])),
      provideHttpClientTesting(),
    ],
  });
  return {
    service: TestBed.inject(AuthApiService),
    http: TestBed.inject(HttpTestingController),
  };
}

describe('AuthApiService', () => {
  beforeEach(() => localStorage.clear());

  it('maps the login response to a session with the client user model', () => {
    const { service, http } = setup();
    let session: unknown;

    service
      .login({ email: 'ada@example.com', password: 'clave-prueba' })
      .subscribe((value) => (session = value));

    const request = http.expectOne('/api/v1/auth/login');
    expect(request.request.body).toEqual({ email: 'ada@example.com', password: 'clave-prueba' });
    expect(request.request.headers.has('Authorization')).toBe(false);
    request.flush({ data: { token: 'tok', user: USER_RESPONSE } });

    expect(session).toEqual({
      token: 'tok',
      user: {
        id: 'u-1',
        name: 'Ada',
        email: 'ada@example.com',
        role: 'user',
        teamId: null,
        displayName: '',
        jobTitle: '',
        department: '',
        avatarKey: null,
        timezone: 'UTC',
        createdAt: '2026-01-01T00:00:00Z',
        lastLoginAt: null,
        emailVerifiedAt: null,
        firstName: '',
        lastName: '',
        locale: 'es',
        phone: null,
        pendingConsents: [],
      },
    });
    http.verify();
  });

  it('sends the explicit token on the account request even when a session is stored', () => {
    const { service, http } = setup();
    TestBed.inject(SessionService).startSession('stored-token');

    service.me('explicit-token').subscribe();

    const request = http.expectOne('/api/v1/users/me');
    expect(request.request.headers.get('Authorization')).toBe('Bearer explicit-token');
    request.flush({ data: USER_RESPONSE });
    http.verify();
  });

  it('does not end a stored session when the explicit token is rejected', () => {
    const { service, http } = setup();
    const session = TestBed.inject(SessionService);
    session.startSession('stored-token');
    let failure: unknown;

    service.me('explicit-token').subscribe({ error: (error: unknown) => (failure = error) });
    http.expectOne('/api/v1/users/me').flush(null, { status: 401, statusText: 'Unauthorized' });

    expect(failure).toMatchObject({ status: 401 });
    expect(session.isAuthenticated()).toBe(true);
    http.verify();
  });

  it('registers and then logs in with the same credentials', () => {
    const { service, http } = setup();
    let session: { token: string } | undefined;

    service
      .register({
        firstName: 'Ada',
        lastName: 'Lovelace',
        phone: { country: 'PE', number: '912345678' },
        email: 'ada@example.com',
        password: 'clave-prueba',
        passwordConfirmation: 'clave-prueba',
        consents: [{ slug: 'privacy_policy', version: 1 }],
      })
      .subscribe((value) => (session = value));

    const register = http.expectOne('/api/v1/auth/register');
    expect(register.request.body).toEqual({
      first_name: 'Ada',
      last_name: 'Lovelace',
      email: 'ada@example.com',
      password: 'clave-prueba',
      password_confirmation: 'clave-prueba',
      phone: { country: 'PE', number: '912345678' },
      locale: 'es',
      consents: [{ slug: 'privacy_policy', version: 1 }],
    });
    register.flush({ data: USER_RESPONSE });
    http
      .expectOne('/api/v1/auth/login')
      .flush({ data: { token: 'tok-nuevo', user: USER_RESPONSE } });

    expect(session?.token).toBe('tok-nuevo');
    http.verify();
  });

  it('sends a partial profile update with only the given fields', () => {
    const { service, http } = setup();

    service.updateProfile({ name: 'Ada', jobTitle: 'Analista', department: 'Ops' }).subscribe();

    const request = http.expectOne({ method: 'PATCH', url: '/api/v1/users/me' });
    expect(request.request.body).toEqual({ name: 'Ada', job_title: 'Analista', department: 'Ops' });
    request.flush({ data: USER_RESPONSE });
    http.verify();
  });

  it('keeps an explicit empty string in the partial update', () => {
    const { service, http } = setup();

    service.updateProfile({ displayName: '' }).subscribe();

    const request = http.expectOne({ method: 'PATCH', url: '/api/v1/users/me' });
    expect(request.request.body).toEqual({ display_name: '' });
    request.flush({ data: USER_RESPONSE });
  });

  it('sends the password change with snake_case fields', () => {
    const { service, http } = setup();

    service.changePassword({ currentPassword: 'actual', newPassword: 'nueva' }).subscribe();

    const request = http.expectOne({ method: 'POST', url: '/api/v1/users/me/password' });
    expect(request.request.body).toEqual({
      current_password: 'actual',
      new_password: 'nueva',
      new_password_confirmation: 'nueva',
    });
    request.flush(null);
    http.verify();
  });
});
