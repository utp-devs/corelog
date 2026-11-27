/** Landing page used when no safe target is given. Matches the workspace home route. */
export const DEFAULT_AUTH_LANDING = '/workspace/';

/**
 * Returns the `returnUrl` value when it is an in-app path, otherwise the fallback landing page.
 * Rejects absolute URLs, protocol-relative paths ("//host") and backslash variants so the
 * parameter cannot send users off-site.
 */
export function safeRedirect(
  value: string | null,
  fallback: string = DEFAULT_AUTH_LANDING,
): string {
  if (!value || !value.startsWith('/') || value.startsWith('//') || value.startsWith('/\\')) {
    return fallback;
  }
  return value;
}

/** Builds the login or register URL that returns to `target` after authentication. */
export function authPathWithRedirect(base: '/login' | '/register', target: string | null): string {
  if (!target) return base;
  return `${base}?returnUrl=${encodeURIComponent(target)}`;
}
