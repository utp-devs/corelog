/**
 * Router path of the issue area, relative to the application base href. The workspace app serves
 * it at /workspace/issues; the base is added by the browser, so routerLink and navigateByUrl must
 * use this path without it.
 */
export const ISSUES_PATH = '/issues';

/** Router path of one issue. Use with routerLink or navigateByUrl, never with a full-page navigation. */
export function issuePath(id: string): string {
  return `${ISSUES_PATH}/${id}`;
}
