import type { ApiError } from '../models';

/** "1 minuto", "45 segundos", "2 horas": a wait written for a person. */
export function waitText(seconds: number): string {
  const total = Math.max(1, Math.ceil(seconds));
  if (total < 60) return total === 1 ? '1 segundo' : `${total} segundos`;
  const minutes = Math.ceil(total / 60);
  if (minutes < 60) return minutes === 1 ? '1 minuto' : `${minutes} minutos`;
  const hours = Math.ceil(minutes / 60);
  return hours === 1 ? '1 hora' : `${hours} horas`;
}

/** True for the answers that ask the person to wait: too many requests and a locked account. */
export function isRateLimited(error: ApiError): boolean {
  return (
    error.status === 429 ||
    error.code === 'auth.rate_limited' ||
    error.code === 'auth.account_locked'
  );
}

/** Spanish message for a rate limit or a lockout, with the wait when the server sent one. */
export function rateLimitMessage(error: ApiError): string {
  const wait = error.retryAfterSeconds;
  const suffix =
    wait !== undefined && wait > 0
      ? ` Inténtalo de nuevo en ${waitText(wait)}.`
      : ' Espera un momento e inténtalo de nuevo.';
  if (error.code === 'auth.account_locked') {
    return `Tu cuenta está bloqueada temporalmente por demasiados intentos fallidos.${suffix}`;
  }
  return `Demasiados intentos seguidos.${suffix}`;
}
