import { issueKeyOf } from '@corelog/core';

/** Readable short key of an issue: the prefix and the first six characters of its identifier. */
export function shortKeyOf(issueId: string): string {
  const full = issueKeyOf(issueId);
  const dash = full.indexOf('-');
  return dash < 0 ? full : full.slice(0, dash + 7);
}
