const DEFAULT_LANDING = '/issues'

/**
 * Returns the `redirect` query value when it is an in-app path, otherwise the default landing page.
 * Rejects absolute URLs and protocol-relative paths ("//host") so the param cannot send users off-site.
 */
export function safeRedirect(value: string | null): string {
  if (!value || !value.startsWith('/') || value.startsWith('//') || value.startsWith('/\\')) {
    return DEFAULT_LANDING
  }
  return value
}

/** Builds the login or register URL that returns to `target` after authentication. */
export function authPathWithRedirect(base: '/login' | '/register', target: string | null): string {
  if (!target) return base
  return `${base}?redirect=${encodeURIComponent(target)}`
}
