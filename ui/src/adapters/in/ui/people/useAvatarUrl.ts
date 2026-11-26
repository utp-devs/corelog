import { useEffect, useState } from 'react'

import { AvatarHttpRepository } from '@/adapters/out/http/avatar-http-repository'
import type { AvatarRepositoryPort } from '@/core/people/application/ports/avatar-repository.port'

interface CacheEntry {
  /** Object URL once the blob arrived; null when the fetch failed. */
  url: string | null
  refs: number
  revoked: boolean
  promise: Promise<void>
}

const cache = new Map<string, CacheEntry>()
const repository: AvatarRepositoryPort = new AvatarHttpRepository()

function keyOf(userId: string, version: string): string {
  return `${userId}|${version}`
}

function revoke(entry: CacheEntry) {
  if (entry.url && !entry.revoked) {
    URL.revokeObjectURL(entry.url)
  }
  entry.revoked = true
  entry.url = null
}

function createEntry(key: string, userId: string): CacheEntry {
  const entry: CacheEntry = { url: null, refs: 0, revoked: false, promise: Promise.resolve() }
  entry.promise = repository
    .getAvatar(userId)
    .then((blob) => {
      // Released before the blob arrived: nobody will use it, so the URL is not kept.
      if (cache.get(key) !== entry) return
      entry.url = URL.createObjectURL(blob)
    })
    .catch(() => {
      entry.url = null
    })
  return entry
}

function release(key: string, entry: CacheEntry) {
  entry.refs -= 1
  if (entry.refs > 0) return
  if (cache.get(key) === entry) cache.delete(key)
  revoke(entry)
}

interface LoadedUrl {
  key: string
  url: string | null
}

/**
 * Object URL of a user's avatar, fetched with the API client (the endpoint needs the JWT, which <img> cannot send).
 * Shared between components for the same user and version; the URL is revoked when the last user unmounts.
 * `version` changes when the avatar changes (pass the user's avatar key) so the image is fetched again.
 * Returns null while loading, when there is no avatar, or when the fetch fails (show initials then).
 */
export function useAvatarUrl(userId: string, hasAvatar: boolean, version = ''): string | null {
  const key = keyOf(userId, version)
  const [loaded, setLoaded] = useState<LoadedUrl | null>(null)

  useEffect(() => {
    if (!hasAvatar) return
    let entry = cache.get(key)
    if (!entry) {
      entry = createEntry(key, userId)
      cache.set(key, entry)
    }
    const current = entry
    current.refs += 1
    let active = true
    void current.promise.then(() => {
      if (active) setLoaded({ key, url: current.url })
    })
    return () => {
      active = false
      release(key, current)
    }
  }, [key, userId, hasAvatar])

  // A result that belongs to another user or version is never shown.
  return hasAvatar && loaded?.key === key ? loaded.url : null
}
