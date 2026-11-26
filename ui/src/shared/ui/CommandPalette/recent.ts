import { useEffect } from 'react'

import type { SearchHit } from '@/shared/types/search'

/** localStorage key for the recently visited projects and issues (JSON array, newest first). */
export const RECENT_STORAGE_KEY = 'corelog.recent'
export const RECENT_LIMIT = 10

export type RecentItem = SearchHit

function isRecentItem(value: unknown): value is RecentItem {
  if (typeof value !== 'object' || value === null) return false
  const candidate = value as Record<string, unknown>
  return (
    (candidate.kind === 'project' || candidate.kind === 'issue') &&
    typeof candidate.id === 'string' &&
    typeof candidate.title === 'string' &&
    typeof candidate.subtitle === 'string' &&
    typeof candidate.link === 'string'
  )
}

/** Recent items, newest first. Invalid or unavailable storage yields an empty list. */
export function getRecent(): RecentItem[] {
  try {
    const parsed: unknown = JSON.parse(window.localStorage.getItem(RECENT_STORAGE_KEY) ?? '[]')
    return Array.isArray(parsed) ? parsed.filter(isRecentItem).slice(0, RECENT_LIMIT) : []
  } catch {
    return []
  }
}

/** Moves the item to the top, removing any earlier entry for the same kind and id. Keeps the last 10. */
export function pushRecent(item: RecentItem): void {
  const entry: RecentItem = {
    kind: item.kind,
    id: item.id,
    title: item.title,
    subtitle: item.subtitle,
    link: item.link,
  }
  const rest = getRecent().filter((existing) => !(existing.kind === entry.kind && existing.id === entry.id))
  try {
    window.localStorage.setItem(RECENT_STORAGE_KEY, JSON.stringify([entry, ...rest].slice(0, RECENT_LIMIT)))
  } catch {
    // Storage unavailable (private mode or quota): recents are a convenience, so nothing else happens.
  }
}

/**
 * Records the visited project or issue. Route components call it with the item they show;
 * passing null or undefined (while loading) records nothing.
 */
export function useTrackRecent(item: RecentItem | null | undefined): void {
  const kind = item?.kind
  const id = item?.id
  const title = item?.title
  const subtitle = item?.subtitle
  const link = item?.link

  useEffect(() => {
    if (kind === undefined || id === undefined || title === undefined || subtitle === undefined || link === undefined) {
      return
    }
    pushRecent({ kind, id, title, subtitle, link })
  }, [kind, id, title, subtitle, link])
}
