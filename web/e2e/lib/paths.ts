/**
 * Maps the application-neutral paths used by the specs onto the URLs served by the three Angular apps
 * behind one origin: portal at "/", workspace at "/workspace/", admin at "/admin/".
 * Specs always pass the neutral path; this is the only place that knows the mount points.
 */

/** Pages served by the portal at the origin root (sign-in, registration, invitation, onboarding). */
const PORTAL_PATHS = ['/login', '/register', '/invite', '/onboarding'] as const;

/** Work pages served by the workspace app under /workspace. */
const WORKSPACE_PATHS = [
  '/issues',
  '/browse',
  '/projects',
  '/dashboard',
  '/teams',
  '/people',
  '/profile',
] as const;

/** Organization administration served by the admin app under /admin. */
const ADMIN_PATHS = ['/organization'] as const;

/** True when `path` is `base` or lies below it (segment boundary, query strings ignored). */
function isUnder(path: string, base: string): boolean {
  const pathname = path.split('?')[0] ?? path;
  return pathname === base || pathname.startsWith(`${base}/`);
}

/** Returns the path (relative to the origin) under which the Angular build serves `neutralPath`. */
export function appPath(neutralPath: string): string {
  if (PORTAL_PATHS.some((base) => isUnder(neutralPath, base))) return neutralPath;
  if (ADMIN_PATHS.some((base) => isUnder(neutralPath, base))) return `/admin${neutralPath}`;
  if (WORKSPACE_PATHS.some((base) => isUnder(neutralPath, base))) return `/workspace${neutralPath}`;
  return neutralPath;
}
