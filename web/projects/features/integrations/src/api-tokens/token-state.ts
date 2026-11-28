import type { ApiToken, ApiTokenScope } from '@corelog/core';

export type TokenState = 'active' | 'expired' | 'revoked';

/** A revoked token is revoked even when it also expired; an expired one stops working without being revoked. */
export function tokenState(token: ApiToken, now: Date = new Date()): TokenState {
  if (token.revokedAt) return 'revoked';
  if (token.expiresAt && new Date(token.expiresAt).getTime() <= now.getTime()) return 'expired';
  return 'active';
}

export const TOKEN_STATE_LABELS: Readonly<Record<TokenState, string>> = {
  active: 'Activo',
  expired: 'Caducado',
  revoked: 'Revocado',
};

export const SCOPE_LABELS: Readonly<Record<ApiTokenScope, string>> = {
  read: 'Solo lectura',
  write: 'Lectura y escritura',
  admin: 'Administración',
};

/** End of the chosen day in local time as an ISO instant, or null when no date was chosen. */
export function expiryFromDate(date: string | null | undefined): string | null {
  if (!date) return null;
  const [year, month, day] = date.split('-').map(Number);
  if (!year || !month || !day) return null;
  return new Date(year, month - 1, day, 23, 59, 59).toISOString();
}

/** The `yyyy-mm-dd` of tomorrow, the earliest expiry the form accepts. */
export function tomorrow(now: Date = new Date()): string {
  const next = new Date(now.getFullYear(), now.getMonth(), now.getDate() + 1);
  const pad = (n: number): string => String(n).padStart(2, '0');
  return `${next.getFullYear()}-${pad(next.getMonth() + 1)}-${pad(next.getDate())}`;
}
