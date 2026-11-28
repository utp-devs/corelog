/**
 * Session id carried by a login token (the `jti` claim), or null when the token is not a JWT or has
 * none. The token is only decoded, never verified: the server is the authority.
 */
export function sessionIdOfToken(token: string | null): string | null {
  const payload = token?.split('.')[1];
  if (!payload) return null;
  try {
    const base64 = payload.replace(/-/g, '+').replace(/_/g, '/');
    const padded = base64 + '='.repeat((4 - (base64.length % 4)) % 4);
    const json = new TextDecoder().decode(Uint8Array.from(atob(padded), (c) => c.charCodeAt(0)));
    const claims = JSON.parse(json) as { jti?: unknown };
    return typeof claims.jti === 'string' && claims.jti ? claims.jti : null;
  } catch {
    return null;
  }
}
