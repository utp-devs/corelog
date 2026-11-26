import { useEffect, useState } from 'react'

import { apiClient } from '@/adapters/out/http/api-client'

/**
 * Fetches the organization logo with the session (an <img> cannot send the JWT). Resolves to null when
 * the request fails: the caller shows initials.
 */
export async function fetchOrgLogo(orgId: string): Promise<Blob | null> {
  try {
    const { data } = await apiClient.get<Blob>(`/organizations/${encodeURIComponent(orgId)}/logo`, { responseType: 'blob' })
    return data
  } catch {
    return null
  }
}

interface LoadedLogo {
  orgId: string
  revision: number
  url: string | null
}

/**
 * Object URL of the organization logo, or null while loading, when the organization has no logo (no request is
 * sent) or when the fetch fails. `revision` changes after an upload or removal so the image is fetched again.
 * Revoked on unmount or when replaced.
 */
export function useOrgLogoUrl(orgId: string, hasLogo: boolean, revision = 0): string | null {
  const [loaded, setLoaded] = useState<LoadedLogo | null>(null)

  useEffect(() => {
    if (!hasLogo) return
    let active = true
    let objectUrl: string | null = null
    void fetchOrgLogo(orgId).then((blob) => {
      if (!active) return
      objectUrl = blob ? URL.createObjectURL(blob) : null
      setLoaded({ orgId, revision, url: objectUrl })
    })
    return () => {
      active = false
      if (objectUrl) URL.revokeObjectURL(objectUrl)
    }
  }, [orgId, hasLogo, revision])

  // A logo that belongs to another organization or revision is never shown.
  return hasLogo && loaded?.orgId === orgId && loaded.revision === revision ? loaded.url : null
}
