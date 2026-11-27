/** Absolute path of the issue area. The workspace app mounts the tickets routes here. */
export const ISSUES_BASE_PATH = '/workspace/issues';

/** Absolute path of one issue. */
export function issuePath(id: string): string {
  return `${ISSUES_BASE_PATH}/${id}`;
}
