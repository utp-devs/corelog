import { initialsOf } from '@/shared/ui/initials'
import { paletteIndex } from '@/shared/ui/Avatar/avatarColor'

import type { OrganizationLogoPort } from '@/core/organizations/application/ports/organization-logo.port'

import { defaultLogoRepository, useOrgLogoUrl } from './org-logo'
import styles from './OrgLogo.module.css'

type OrgLogoProps = {
  orgId: string
  /** Organization name: origin of the initials shown when there is no logo. */
  name: string
  /** Organization.hasLogo. False means no request is made and the initials are shown. */
  hasLogo: boolean
  /** Changes after an upload or removal so the logo is fetched again. */
  revision?: number
  size?: 24 | 32
  /** Port that fetches the logo; defaults to the HTTP repository. */
  repository?: OrganizationLogoPort
}

/** Organization logo fetched with the session, or a colored tile with the initials of the name (ProjectAvatar style). */
export function OrgLogo({ orgId, name, hasLogo, revision = 0, size = 24, repository = defaultLogoRepository }: OrgLogoProps) {
  const src = useOrgLogoUrl(repository, orgId, hasLogo, revision)
  const sizeClass = styles[`size${size}`]

  if (src) {
    return <img src={src} alt="" className={`${styles.image} ${sizeClass}`} data-testid="org-logo" />
  }

  return (
    <span aria-hidden="true" className={`${styles.tile} ${sizeClass}`} data-palette={paletteIndex(orgId)} data-testid="org-logo-fallback">
      {initialsOf(name)}
    </span>
  )
}
