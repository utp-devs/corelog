import { Injectable, inject } from '@angular/core';
import { provideHttpClient, withInterceptors } from '@angular/common/http';
import { HttpTestingController, provideHttpClientTesting } from '@angular/common/http/testing';
import { TestBed } from '@angular/core/testing';

import { provideCorelogTesting } from '../../testing';
import { authInterceptor } from '../interceptors/auth.interceptor';
import { errorInterceptor } from '../interceptors/error.interceptor';
import { AUTH_TOKEN_KEY, SessionService } from '../services/session.service';
import { AuthStore } from './auth.store';
import { resetOnLogout } from './session-scoped';

const USER_RESPONSE = {
  id: 'u-1',
  name: 'Ada',
  email: 'ada@example.com',
  role: 'admin',
  created_at: '2026-01-01T00:00:00Z',
};

const LOGIN_RESPONSE = { data: { token: 'sample-jwt-token', user: USER_RESPONSE } };

/** Store that counts how often it was reset because the session ended. */
@Injectable({ providedIn: 'root' })
class SessionScopedProbe {
  resets = 0;

  constructor() {
    resetOnLogout(() => {
      this.resets += 1;
    });
  }
}

function setup(): HttpTestingController {
  TestBed.configureTestingModule({
    providers: [
      provideCorelogTesting(),
      provideHttpClient(withInterceptors([authInterceptor, errorInterceptor])),
      provideHttpClientTesting(),
    ],
  });
  return TestBed.inject(HttpTestingController);
}

async function signIn(http: HttpTestingController): Promise<AuthStore> {
  const store = TestBed.inject(AuthStore);
  const pending = store.login({ email: 'ada@example.com', password: 'clave-prueba' });
  http.expectOne('/api/v1/auth/login').flush(LOGIN_RESPONSE);
  await pending;
  return store;
}

describe('AuthStore', () => {
  beforeEach(() => localStorage.clear());

  it('initializes with default state', () => {
    setup();
    const store = TestBed.inject(AuthStore);
    expect(store.user()).toBeNull();
    expect(store.status()).toBe('idle');
    expect(store.error()).toBeNull();
    expect(store.sessionChecked()).toBe(false);
  });

  it('clears session and token on logout', async () => {
    const http = setup();
    const store = await signIn(http);
    expect(store.user()?.name).toBe('Ada');

    store.logout();
    http.expectOne('/api/v1/auth/logout').flush(null);

    expect(store.user()).toBeNull();
    expect(store.status()).toBe('idle');
    expect(store.sessionChecked()).toBe(true);
    expect(localStorage.getItem(AUTH_TOKEN_KEY)).toBeNull();
    http.verify();
  });

  it('resets the stores that depend on the session when it ends', async () => {
    const http = setup();
    const probe = TestBed.inject(SessionScopedProbe);
    TestBed.flushEffects();
    const store = await signIn(http);
    TestBed.flushEffects();
    const resetsWhileSignedIn = probe.resets;

    store.logout();
    http.expectOne('/api/v1/auth/logout').flush(null);
    TestBed.flushEffects();

    expect(probe.resets).toBeGreaterThan(resetsWhileSignedIn);
  });

  it('sends the server logout with the token that was active', async () => {
    const http = setup();
    const store = await signIn(http);

    store.logout();

    const request = http.expectOne('/api/v1/auth/logout');
    expect(request.request.method).toBe('POST');
    expect(request.request.headers.get('Authorization')).toBe('Bearer sample-jwt-token');
    request.flush(null);
  });

  it('stores the token and the account after a login', async () => {
    const http = setup();
    const store = await signIn(http);

    expect(localStorage.getItem(AUTH_TOKEN_KEY)).toBe('sample-jwt-token');
    expect(store.status()).toBe('authenticated');
    expect(store.sessionChecked()).toBe(true);
    expect(store.isAuthenticated()).toBe(true);
    expect(store.isPlatformAdmin()).toBe(true);
    expect(store.displayName()).toBe('Ada');
    http.verify();
  });

  it('keeps the API message and rethrows when the login is rejected', async () => {
    const http = setup();
    const store = TestBed.inject(AuthStore);

    const pending = store.login({ email: 'ada@example.com', password: 'mala' });
    http
      .expectOne('/api/v1/auth/login')
      .flush(
        { error: { message: 'Correo o contraseña no válidos', code: 'auth.invalid_credentials' } },
        { status: 401, statusText: 'Unauthorized' },
      );

    await expect(pending).rejects.toMatchObject({ status: 401, code: 'auth.invalid_credentials' });
    expect(store.status()).toBe('error');
    expect(store.error()).toBe('Correo o contraseña no válidos');
    expect(store.sessionChecked()).toBe(true);
    expect(localStorage.getItem(AUTH_TOKEN_KEY)).toBeNull();
    http.verify();
  });

  it('registers the account and then signs in with the same credentials', async () => {
    const http = setup();
    const store = TestBed.inject(AuthStore);

    const pending = store.register({
      name: 'Ada',
      email: 'ada@example.com',
      password: 'clave-prueba',
    });
    http.expectOne('/api/v1/auth/register').flush({ data: USER_RESPONSE });
    const login = http.expectOne('/api/v1/auth/login');
    expect(login.request.body).toEqual({ email: 'ada@example.com', password: 'clave-prueba' });
    login.flush(LOGIN_RESPONSE);
    await pending;

    expect(store.user()?.email).toBe('ada@example.com');
    expect(store.status()).toBe('authenticated');
    expect(localStorage.getItem(AUTH_TOKEN_KEY)).toBe('sample-jwt-token');
    http.verify();
  });

  it('restores the account from the stored token', async () => {
    const http = setup();
    TestBed.inject(SessionService).startSession('stored-token');
    const store = TestBed.inject(AuthStore);

    const pending = store.restoreSession();
    const request = http.expectOne('/api/v1/users/me');
    expect(request.request.headers.get('Authorization')).toBe('Bearer stored-token');
    request.flush({ data: USER_RESPONSE });
    await pending;

    expect(store.user()?.id).toBe('u-1');
    expect(store.status()).toBe('authenticated');
    expect(store.sessionChecked()).toBe(true);
    expect(localStorage.getItem(AUTH_TOKEN_KEY)).toBe('stored-token');
    http.verify();
  });

  it('discards an invalid stored token without redirecting', async () => {
    const http = setup();
    const session = TestBed.inject(SessionService);
    session.startSession('expired-token');
    const store = TestBed.inject(AuthStore);

    const pending = store.restoreSession();
    http
      .expectOne('/api/v1/users/me')
      .flush(
        { error: { message: 'Sesión expirada' } },
        { status: 401, statusText: 'Unauthorized' },
      );
    await pending;

    expect(localStorage.getItem(AUTH_TOKEN_KEY)).toBeNull();
    expect(session.isAuthenticated()).toBe(false);
    expect(store.user()).toBeNull();
    expect(store.status()).toBe('idle');
    expect(store.sessionChecked()).toBe(true);
    http.verify();
  });

  it('marks the session as checked without a request when no token is stored', async () => {
    const http = setup();
    const store = TestBed.inject(AuthStore);

    await store.restoreSession();

    expect(store.status()).toBe('idle');
    expect(store.sessionChecked()).toBe(true);
    http.expectNone('/api/v1/users/me');
  });

  it('replaces the account after a profile update and keeps it on failure', async () => {
    const http = setup();
    const store = await signIn(http);

    const pending = store.updateProfile({ displayName: 'Ada L.' });
    const request = http.expectOne({ method: 'PATCH', url: '/api/v1/users/me' });
    expect(request.request.body).toEqual({ display_name: 'Ada L.' });
    request.flush({ data: { ...USER_RESPONSE, display_name: 'Ada L.' } });
    await pending;

    expect(store.displayName()).toBe('Ada L.');

    const failing = store.updateProfile({ jobTitle: 'x' });
    http
      .expectOne({ method: 'PATCH', url: '/api/v1/users/me' })
      .flush(
        { error: { message: 'Demasiado largo' } },
        { status: 422, statusText: 'Unprocessable' },
      );
    await expect(failing).rejects.toMatchObject({ status: 422 });
    expect(store.displayName()).toBe('Ada L.');
    http.verify();
  });

  it('sends the password change and rejects when the server refuses it', async () => {
    const http = setup();
    const store = await signIn(http);

    const pending = store.changePassword({
      currentPassword: 'clave-prueba',
      newPassword: 'nueva-clave',
    });
    const request = http.expectOne('/api/v1/users/me/password');
    expect(request.request.body).toEqual({
      current_password: 'clave-prueba',
      new_password: 'nueva-clave',
    });
    request.flush(null);
    await pending;

    const failing = store.changePassword({ currentPassword: 'mala', newPassword: 'nueva-clave' });
    http
      .expectOne('/api/v1/users/me/password')
      .flush(
        { error: { message: 'Contraseña actual incorrecta' } },
        { status: 400, statusText: 'Bad Request' },
      );
    await expect(failing).rejects.toMatchObject({ status: 400 });
    http.verify();
  });

  it('derives the display name from the name when no display name is set', async () => {
    const http = setup();
    const store = await signIn(http);
    expect(store.displayName()).toBe('Ada');
    expect(store.isPlatformAdmin()).toBe(true);
    store.logout();
    http.expectOne('/api/v1/auth/logout').flush(null);
    expect(store.displayName()).toBeNull();
    expect(store.isPlatformAdmin()).toBe(false);
  });
});
