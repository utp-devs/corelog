import type { ApiToken } from '@corelog/core';

import { expiryFromDate, tokenState, tomorrow } from './token-state';

const token = (patch: Partial<ApiToken> = {}): ApiToken => ({
  id: 't1',
  name: 'CI',
  prefix: 'cl_pat_ab12',
  scopes: ['read'],
  expiresAt: null,
  lastUsedAt: null,
  revokedAt: null,
  createdAt: '2026-01-01T00:00:00Z',
  ...patch,
});

describe('tokenState', () => {
  const now = new Date('2026-06-01T12:00:00Z');

  it('is active without expiry or with a future one', () => {
    expect(tokenState(token(), now)).toBe('active');
    expect(tokenState(token({ expiresAt: '2026-07-01T00:00:00Z' }), now)).toBe('active');
  });

  it('is expired once the date passed', () => {
    expect(tokenState(token({ expiresAt: '2026-05-01T00:00:00Z' }), now)).toBe('expired');
  });

  it('is revoked even when it also expired', () => {
    expect(
      tokenState(
        token({ revokedAt: '2026-02-01T00:00:00Z', expiresAt: '2026-03-01T00:00:00Z' }),
        now,
      ),
    ).toBe('revoked');
  });
});

describe('expiry dates', () => {
  it('turns a day into the end of that day', () => {
    const iso = expiryFromDate('2030-03-15');
    const date = new Date(iso as string);
    expect([date.getFullYear(), date.getMonth(), date.getDate(), date.getHours()]).toEqual([
      2030, 2, 15, 23,
    ]);
  });

  it('returns null for an empty date', () => {
    expect(expiryFromDate('')).toBeNull();
    expect(expiryFromDate(null)).toBeNull();
  });

  it('computes tomorrow across month ends', () => {
    expect(tomorrow(new Date(2026, 0, 31))).toBe('2026-02-01');
  });
});
