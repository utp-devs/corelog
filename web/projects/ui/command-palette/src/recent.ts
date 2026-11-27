import { pushRecent, type RecentItem } from '@corelog/core';

/**
 * Records a visited project or issue in the recent list. Route components call it with the item
 * they show. Passing null or undefined (while loading) records nothing.
 */
export function trackRecent(item: RecentItem | null | undefined): void {
  if (!item) {
    return;
  }
  pushRecent(item);
}
