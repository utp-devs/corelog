import { paletteIndex } from '../Avatar/avatarColor'
import { initialsOf } from '../initials'
import styles from './TeamChip.module.css'

type TeamChipProps = {
  /** Identificador estable del equipo: decide el color. */
  id: string
  name: string
  memberCount?: number
  /** Si existe, el chip es un enlace al equipo. */
  href?: string
  'data-testid'?: string
}

function memberText(count: number): string {
  return count === 1 ? '1 miembro' : `${count} miembros`
}

/** Equipo con cuadrado de color, iniciales, nombre y número de miembros. */
export function TeamChip({ id, name, memberCount, href, 'data-testid': testId }: TeamChipProps) {
  const content = (
    <>
      <span aria-hidden="true" className={styles.tile} data-palette={paletteIndex(id)}>
        {initialsOf(name)}
      </span>
      <span className={styles.text}>
        <span className={styles.name}>{name}</span>
        {memberCount !== undefined && <span className={styles.meta}>{memberText(memberCount)}</span>}
      </span>
    </>
  )

  if (href) {
    return (
      <a href={href} className={styles.chip} data-testid={testId}>
        {content}
      </a>
    )
  }
  return (
    <span className={styles.chip} data-testid={testId}>
      {content}
    </span>
  )
}
