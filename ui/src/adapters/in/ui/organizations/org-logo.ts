import { useEffect, useState } from 'react'

import { apiClient } from '@/adapters/out/http/api-client'

/**
 * Fetches the organization logo with the session (an <img> cannot send the JWT). Resolves to null when
 * the organization has no logo (404 org.logo_not_found) or the request fails: the caller shows initials.
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
  url: string | null
}

/** Object URL of the organization logo, or null while loading or when there is none. Revoked on unmount. */
export function useOrgLogoUrl(orgId: string): string | null {
  const [loaded, setLoaded] = useState<LoadedLogo | null>(null)

  useEffect(() => {
    let active = true
    let objectUrl: string | null = null
    void fetchOrgLogo(orgId).then((blob) => {
      if (!active) return
      objectUrl = blob ? URL.createObjectURL(blob) : null
      setLoaded({ orgId, url: objectUrl })
    })
    return () => {
      active = false
      if (objectUrl) URL.revokeObjectURL(objectUrl)
    }
  }, [orgId])

  // A logo that belongs to another organization is never shown.
  return loaded?.orgId === orgId ? loaded.url : null
}
