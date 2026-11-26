import { useEffect, useState } from 'react'

import { OrganizationHttpRepository } from '@/adapters/out/http/organization-http-repository'
import type { OrganizationLogoPort } from '@/core/organizations/application/ports/organization-logo.port'

/** Default repository for the logo tiles; tests and other callers inject their own through the OrgLogo prop. */
export const defaultLogoRepository: OrganizationLogoPort = new OrganizationHttpRepository()

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
export function useOrgLogoUrl(
  repository: OrganizationLogoPort,
  orgId: string,
  hasLogo: boolean,
  revision = 0,
): string | null {
  const [loaded, setLoaded] = useState<LoadedLogo | null>(null)

  useEffect(() => {
    if (!hasLogo) return
    let active = true
    let objectUrl: string | null = null
    void repository.fetchLogoBlob(orgId).then((blob) => {
      if (!active) return
      objectUrl = blob ? URL.createObjectURL(blob) : null
      setLoaded({ orgId, revision, url: objectUrl })
    })
    return () => {
      active = false
      if (objectUrl) URL.revokeObjectURL(objectUrl)
    }
  }, [repository, orgId, hasLogo, revision])

  // A logo that belongs to another organization or revision is never shown.
  return hasLogo && loaded?.orgId === orgId && loaded.revision === revision ? loaded.url : null
}
