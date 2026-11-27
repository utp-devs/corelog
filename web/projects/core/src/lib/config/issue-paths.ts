/**
 * Router path of the issue list, relative to the application base href. The workspace app serves
 * it at /workspace/issues; the base is added by the browser, so routerLink and navigateByUrl must
 * use this path without it.
 */
export const ISSUES_PATH = '/issues';

/** Router path prefix of one issue, addressed by its key (CORE-12) or its UUID. */
export const BROWSE_PATH = '/browse';

/**
 * Router path of one issue. Pass the key (preferred) or the id. Use with routerLink or
 * navigateByUrl, never with a full-page navigation.
 */
export function issuePath(keyOrId: string): string {
  return `${BROWSE_PATH}/${keyOrId}`;
}

const LEGACY_ISSUE_LINK = /^\/issues\/(?!new(?:[/?#]|$))([^/?#]+)(.*)$/;

/**
 * Links stored by older data or produced by the server point at /issues/<id>. They are rewritten
 * to the router path of the issue (/browse/<id>); any other link is returned unchanged.
 */
export function normalizeIssueLink(link: string): string {
  const match = LEGACY_ISSUE_LINK.exec(link);
  return match ? `${BROWSE_PATH}/${match[1]}${match[2]}` : link;
}
