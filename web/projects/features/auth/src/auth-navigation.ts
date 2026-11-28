import type { Router } from '@angular/router';

/** Paths of the public sign-in area. Targets under these stay inside the portal app. */
const PORTAL_PATHS: readonly string[] = [
  '/login',
  '/register',
  '/forgot-password',
  '/reset-password',
  '/verify-email',
];
const PORTAL_PREFIXES: readonly string[] = ['/invite/'];

/**
 * Moves the user to a safe target after authentication. Portal targets use the router; any other
 * app (workspace, admin) is reached with a full-page navigation through the injected assign function.
 */
export function goAfterAuth(target: string, router: Router, assign: (url: string) => void): void {
  const path = target.split(/[?#]/)[0];
  const insidePortal =
    PORTAL_PATHS.includes(path) || PORTAL_PREFIXES.some((prefix) => path.startsWith(prefix));
  if (insidePortal) {
    void router.navigateByUrl(target);
    return;
  }
  assign(target);
}

/** Moves focus to the first control marked invalid inside the given root element. */
export function focusFirstInvalid(root: HTMLElement): void {
  root.querySelector<HTMLElement>('[aria-invalid="true"]')?.focus();
}
