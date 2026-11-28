import { ApiError } from '@corelog/core';

import { authErrorMessage } from './auth-messages';

function api(patch: Partial<ApiError>): ApiError {
  return { status: 500, code: 'unknown', message: '', details: [], ...patch };
}

describe('authErrorMessage', () => {
  it('maps the credentials code and its English message to Spanish', () => {
    const byCode = api({ status: 401, code: 'auth.invalid_credentials', message: 'x' });
    const byText = api({ status: 401, message: 'invalid email or password' });
    expect(authErrorMessage(byCode, 'login')).toContain('no son correctos');
    expect(authErrorMessage(byText, 'login')).toContain('no son correctos');
  });

  it('maps an email already registered on sign-up', () => {
    expect(
      authErrorMessage(api({ status: 409, message: 'email is already registered' }), 'register'),
    ).toContain('Ya existe una cuenta');
    expect(authErrorMessage(api({ status: 409, code: 'auth.email_taken' }), 'register')).toContain(
      'Ya existe una cuenta',
    );
  });

  it('keeps Spanish messages and capitalizes them', () => {
    expect(authErrorMessage(api({ message: 'el correo no es válido' }), 'login')).toBe(
      'El correo no es válido',
    );
  });

  it('never returns raw English', () => {
    const text = authErrorMessage(api({ status: 500, message: 'database exploded' }), 'login');
    expect(text).toContain('No se pudo iniciar sesión');
    expect(authErrorMessage(api({ status: 0, message: 'Network down' }), 'register')).toContain(
      'conexión',
    );
  });

  it('explains throttling and disabled accounts', () => {
    expect(authErrorMessage(api({ status: 429, message: 'slow down' }), 'login')).toContain(
      'Demasiados intentos',
    );
    expect(authErrorMessage(api({ status: 403, message: 'user is disabled' }), 'login')).toContain(
      'desactivada',
    );
  });

  it('tells the wait of a rate limit, a lockout and an unverified email', () => {
    const limited = api({ status: 429, code: 'auth.rate_limited', retryAfterSeconds: 90 });
    expect(authErrorMessage(limited, 'login')).toContain('2 minutos');
    const locked = api({ status: 423, code: 'auth.account_locked', retryAfterSeconds: 30 });
    expect(authErrorMessage(locked, 'login')).toContain('bloqueada');
    const unverified = api({ status: 403, code: 'auth.email_not_verified', message: 'x' });
    expect(authErrorMessage(unverified, 'login')).toContain('no está verificado');
  });
});
