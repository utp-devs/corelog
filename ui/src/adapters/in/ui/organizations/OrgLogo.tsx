import { initialsOf } from '@/shared/ui/initials'
import { paletteIndex } from '@/shared/ui/Avatar/avatarColor'

import { useOrgLogoUrl } from './org-logo'
import styles from './OrgLogo.module.css'

type OrgLogoProps = {
  orgId: string
  /** Organization name: origin of the initials shown when there is no logo. */
  name: string
  size?: 24 | 32
}

/** Organization logo fetched with the session, or a colored tile with the initials of the name (ProjectAvatar style). */
export function OrgLogo({ orgId, name, size = 24 }: OrgLogoProps) {
  const src = useOrgLogoUrl(orgId)
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
