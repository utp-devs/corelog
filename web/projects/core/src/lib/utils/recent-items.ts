import { SearchHit } from '../models/search.model';

/** localStorage key for the recently visited projects and issues (JSON array, newest first). */
export const RECENT_STORAGE_KEY = 'corelog.recent';
export const RECENT_LIMIT = 10;

export type RecentItem = SearchHit;

function isRecentItem(value: unknown): value is RecentItem {
  if (typeof value !== 'object' || value === null) {
    return false;
  }
  const candidate = value as Record<string, unknown>;
  return (
    (candidate['kind'] === 'project' || candidate['kind'] === 'issue') &&
    typeof candidate['id'] === 'string' &&
    typeof candidate['title'] === 'string' &&
    typeof candidate['subtitle'] === 'string' &&
    typeof candidate['link'] === 'string'
  );
}

/** Recent items, newest first. Invalid or unavailable storage yields an empty list. */
export function getRecent(): RecentItem[] {
  try {
    const parsed: unknown = JSON.parse(localStorage.getItem(RECENT_STORAGE_KEY) ?? '[]');
    return Array.isArray(parsed) ? parsed.filter(isRecentItem).slice(0, RECENT_LIMIT) : [];
  } catch {
    return [];
  }
}

/** Moves the item to the top, removing any earlier entry for the same kind and id. Keeps the last 10. */
export function pushRecent(item: RecentItem): RecentItem[] {
  const entry: RecentItem = {
    kind: item.kind,
    id: item.id,
    title: item.title,
    subtitle: item.subtitle,
    link: item.link,
  };
  const rest = getRecent().filter(
    (existing) => !(existing.kind === entry.kind && existing.id === entry.id),
  );
  const next = [entry, ...rest].slice(0, RECENT_LIMIT);
  try {
    localStorage.setItem(RECENT_STORAGE_KEY, JSON.stringify(next));
  } catch {
    // Storage unavailable (private mode or quota): recents are a convenience, so nothing else happens.
  }
  return next;
}
