import { isRateLimited, rateLimitMessage, waitText } from './rate-limit-copy';

const base = { status: 429, code: 'auth.rate_limited', message: 'x', details: [] };

describe('rate limit copy', () => {
  it('writes waits for people', () => {
    expect(waitText(1)).toBe('1 segundo');
    expect(waitText(45)).toBe('45 segundos');
    expect(waitText(61)).toBe('2 minutos');
    expect(waitText(3600)).toBe('1 hora');
  });

  it('mentions the wait when the server sent it', () => {
    expect(rateLimitMessage({ ...base, retryAfterSeconds: 90 })).toContain('2 minutos');
    expect(rateLimitMessage(base)).toContain('Espera un momento');
  });

  it('tells a lockout apart', () => {
    const locked = { ...base, status: 423, code: 'auth.account_locked', retryAfterSeconds: 30 };
    expect(isRateLimited(locked)).toBe(true);
    expect(rateLimitMessage(locked)).toContain('bloqueada');
    expect(isRateLimited({ ...base, status: 401, code: 'auth.invalid_credentials' })).toBe(false);
  });
});
