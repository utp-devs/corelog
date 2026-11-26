import type { CSSProperties } from 'react'

import { paletteIndex } from '@/shared/ui/Avatar/avatarColor'
import { isHexColor } from '@/shared/ui/ColorPicker/colorValue'
import { initialsOf } from '@/shared/ui/initials'
import styles from './TeamTile.module.css'

type TeamTileProps = {
  id: string
  name: string
  /** Hex color chosen for the team, or null to use the color derived from the id. */
  color: string | null
  size?: 'md' | 'lg'
}

/** Square tile with the team initials in the team color. Decorative: the team name is rendered next to it. */
export function TeamTile({ id, name, color, size = 'md' }: TeamTileProps) {
  const sizeClass = size === 'lg' ? styles.lg : styles.md
  if (color !== null && isHexColor(color)) {
    return (
      <span
        aria-hidden="true"
        className={`${styles.tile} ${sizeClass}`}
        data-custom="true"
        /* runtime-style */
        style={{ '--cl-team-color': color } as CSSProperties}
      >
        {initialsOf(name)}
      </span>
    )
  }
  return (
    <span aria-hidden="true" className={`${styles.tile} ${sizeClass}`} data-palette={paletteIndex(id)}>
      {initialsOf(name)}
    </span>
  )
}
