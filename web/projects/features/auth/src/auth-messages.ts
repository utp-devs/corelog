import { asApiError, toastCopyOf } from '@corelog/core';

export type AuthContext = 'login' | 'register';

const FALLBACKS: Readonly<Record<AuthContext, string>> = {
  login: 'No se pudo iniciar sesión. Revisa tus datos e inténtalo de nuevo.',
  register: 'No se pudo crear la cuenta. Inténtalo de nuevo en unos minutos.',
};

const BAD_CREDENTIALS =
  'El correo o la contraseña no son correctos. Revísalos e inténtalo de nuevo.';
const EMAIL_TAKEN = 'Ya existe una cuenta con este correo. Inicia sesión o usa otro correo.';
const ACCOUNT_DISABLED =
  'Tu cuenta está desactivada. Pide a un administrador de tu organización que la reactive.';
const WEAK_PASSWORD = 'La contraseña es demasiado débil. Usa al menos 8 caracteres.';
const TOO_MANY = 'Demasiados intentos seguidos. Espera un momento e inténtalo de nuevo.';

/** Words and characters that only appear in Spanish text: such a message is already friendly. */
const SPANISH_HINT =
  /[áéíóúñ¿¡]|\b(el|la|los|las|de|del|no|se|un|una|que|por|para|con|tu|tus|correo|contraseña)\b/i;

function capitalized(text: string): string {
  const trimmed = text.trim();
  return trimmed.charAt(0).toUpperCase() + trimmed.slice(1);
}

/**
 * The single place that turns a sign-in or sign-up failure into Spanish copy for the error banner.
 * It maps API codes first, then the English wording the server sends, and never returns raw English.
 */
export function authErrorMessage(error: unknown, context: AuthContext): string {
  const api = asApiError(error);
  const message = api.message.trim();

  if (api.code === 'auth.invalid_credentials') return BAD_CREDENTIALS;
  if (api.code === 'auth.email_taken') return EMAIL_TAKEN;
  if (api.status === 429) return TOO_MANY;

  if (
    /invalid (email|credentials|password)|incorrect|wrong password|bad credentials/i.test(message)
  )
    return BAD_CREDENTIALS;
  if (/already (registered|exists|in use|taken)|duplicate/i.test(message)) return EMAIL_TAKEN;
  if (/disabled|deactivated|inactive|suspended|blocked/i.test(message)) return ACCOUNT_DISABLED;
  if (/password/i.test(message) && /(short|least|weak|characters|length)/i.test(message))
    return WEAK_PASSWORD;

  if (api.status === 401 && context === 'login') return BAD_CREDENTIALS;
  if (api.status === 409 && context === 'register') return EMAIL_TAKEN;

  if (message && SPANISH_HINT.test(message)) return capitalized(message);

  if (api.status === 0 || api.status === 403 || api.status === 404) {
    return toastCopyOf(api, FALLBACKS[context]).detail;
  }
  return FALLBACKS[context];
}
